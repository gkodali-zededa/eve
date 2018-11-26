// Copyright (c) 2017-2018 Zededa, Inc.
// All rights reserved.

package client

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/zededa/api/zmet"
	"github.com/zededa/go-provision/agentlog"
	"github.com/zededa/go-provision/cast"
	"github.com/zededa/go-provision/devicenetwork"
	"github.com/zededa/go-provision/pidfile"
	"github.com/zededa/go-provision/pubsub"
	"github.com/zededa/go-provision/types"
	"github.com/zededa/go-provision/zedcloud"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	agentName   = "zedclient"
	tmpDirname  = "/var/tmp/zededa"
	maxDelay    = time.Second * 600 // 10 minutes
	uuidMaxWait = time.Second * 60  // 1 minute
)

// Really a constant
var nilUUID uuid.UUID

// Set from Makefile
var Version = "No version specified"

// Assumes the config files are in identityDirname, which is /config
// by default. The files are
//  root-certificate.pem	Fixed? Written if redirected. factory-root-cert?
//  server			Fixed? Written if redirected. factory-root-cert?
//  onboard.cert.pem, onboard.key.pem	Per device onboarding certificate/key
//  		   		for selfRegister operation
//  device.cert.pem,
//  device.key.pem		Device certificate/key created before this
//  		     		client is started.
//  uuid			Written by getUuid operation
//  hardwaremodel		Written by getUuid if server returns a hardwaremodel
//
//

type clientContext struct {
	subDeviceNetworkStatus *pubsub.Subscription
	deviceNetworkStatus    *types.DeviceNetworkStatus
	usableAddressCount     int
	subGlobalConfig        *pubsub.Subscription
}

var debug = false
var debugOverride bool // From command line arg

func Run() {
	versionPtr := flag.Bool("v", false, "Version")
	debugPtr := flag.Bool("d", false, "Debug flag")
	forcePtr := flag.Bool("f", false, "Force using onboarding cert")
	dirPtr := flag.String("D", "/config", "Directory with certs etc")
	stdoutPtr := flag.Bool("s", false, "Use stdout instead of console")
	noPidPtr := flag.Bool("p", false, "Do not check for running client")
	maxRetriesPtr := flag.Int("r", 0, "Max ping retries")

	flag.Parse()

	versionFlag := *versionPtr
	debug = *debugPtr
	debugOverride = debug
	if debugOverride {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	forceOnboardingCert := *forcePtr
	identityDirname := *dirPtr
	useStdout := *stdoutPtr
	noPidFlag := *noPidPtr
	maxRetries := *maxRetriesPtr
	args := flag.Args()
	if versionFlag {
		fmt.Printf("%s: %s\n", os.Args[0], Version)
		return
	}
	// XXX json to file; text to stdout/console?
	logf, err := agentlog.Init("client")
	if err != nil {
		log.Fatal(err)
	}
	defer logf.Close()
	// For limited output on console
	consolef := os.Stdout
	if !useStdout {
		consolef, err = os.OpenFile("/dev/console", os.O_RDWR|os.O_APPEND,
			0666)
		if err != nil {
			log.Fatal(err)
		}
	}
	multi := io.MultiWriter(logf, consolef)
	log.SetOutput(multi)
	if !noPidFlag {
		if err := pidfile.CheckAndCreatePidfile(agentName); err != nil {
			log.Fatal(err)
		}
	}
	log.Infof("Starting %s\n", agentName)
	operations := map[string]bool{
		"selfRegister": false,
		"ping":         false,
		"getUuid":      false,
	}
	for _, op := range args {
		if _, ok := operations[op]; ok {
			operations[op] = true
		} else {
			log.Error("Unknown arg %s\n", op)
			log.Fatal("Usage: " + os.Args[0] +
				"[-o] [-d <identityDirname> [<operations>...]]")
		}
	}

	onboardCertName := identityDirname + "/onboard.cert.pem"
	onboardKeyName := identityDirname + "/onboard.key.pem"
	deviceCertName := identityDirname + "/device.cert.pem"
	deviceKeyName := identityDirname + "/device.key.pem"
	serverFileName := identityDirname + "/server"
	uuidFileName := identityDirname + "/uuid"
	hardwaremodelFileName := identityDirname + "/hardwaremodel"

	cms := zedcloud.GetCloudMetrics() // Need type of data
	pub, err := pubsub.Publish(agentName, cms)
	if err != nil {
		log.Fatal(err)
	}

	var oldUUID uuid.UUID
	b, err := ioutil.ReadFile(uuidFileName)
	if err == nil {
		uuidStr := strings.TrimSpace(string(b))
		oldUUID, err = uuid.FromString(uuidStr)
		if err != nil {
			log.Warningf("Malformed UUID file ignored: %s\n", err)
		}
	}
	var oldHardwaremodel string
	b, err = ioutil.ReadFile(hardwaremodelFileName)
	if err == nil {
		oldHardwaremodel = strings.TrimSpace(string(b))
	}

	clientCtx := clientContext{
		deviceNetworkStatus: &types.DeviceNetworkStatus{},
	}

	// Look for global config such as log levels
	subGlobalConfig, err := pubsub.Subscribe("", types.GlobalConfig{},
		false, &clientCtx)
	if err != nil {
		log.Fatal(err)
	}
	subGlobalConfig.ModifyHandler = handleGlobalConfigModify
	subGlobalConfig.DeleteHandler = handleGlobalConfigDelete
	clientCtx.subGlobalConfig = subGlobalConfig
	subGlobalConfig.Activate()

	subDeviceNetworkStatus, err := pubsub.Subscribe("nim",
		types.DeviceNetworkStatus{}, false, &clientCtx)
	if err != nil {
		log.Fatal(err)
	}
	subDeviceNetworkStatus.ModifyHandler = handleDNSModify
	subDeviceNetworkStatus.DeleteHandler = handleDNSDelete
	clientCtx.subDeviceNetworkStatus = subDeviceNetworkStatus
	subDeviceNetworkStatus.Activate()

	// After 5 seconds we check; if we already have a UUID we continue
	// with that one
	t1 := time.NewTimer(5 * time.Second)
	done := clientCtx.usableAddressCount != 0

	// Make sure we wait for a while to process all the DeviceUplinkConfigs
	for clientCtx.usableAddressCount == 0 || !done {
		log.Infof("Waiting for usableAddressCount %d and done %v\n",
			clientCtx.usableAddressCount, done)
		select {
		case change := <-subGlobalConfig.C:
			subGlobalConfig.ProcessChange(change)

		case change := <-subDeviceNetworkStatus.C:
			subDeviceNetworkStatus.ProcessChange(change)

		case <-t1.C:
			done = true
			// If we already know a uuid we can skip
			// This might not set hardwaremodel when upgrading
			// an onboarded system without /config/hardwaremodel.
			// Unlikely to have a network outage during that
			// upgrade *and* require an override.
			if clientCtx.usableAddressCount == 0 &&
				operations["getUuid"] && oldUUID != nilUUID {

				log.Infof("Already have a UUID %s; declaring success\n",
					oldUUID.String())
				// Likely zero metrics
				err := pub.Publish("global", zedcloud.GetCloudMetrics())
				if err != nil {
					log.Errorln(err)
				}
				return
			}
		}
	}
	log.Infof("Got for deviceNetworkConfig: %d addresses\n",
		clientCtx.usableAddressCount)

	// Inform ledmanager that we have uplink addresses
	types.UpdateLedManagerConfig(2)

	zedcloudCtx := zedcloud.ZedCloudContext{
		DeviceNetworkStatus: clientCtx.deviceNetworkStatus,
		FailureFunc:         zedcloud.ZedCloudFailure,
		SuccessFunc:         zedcloud.ZedCloudSuccess,
	}
	var onboardCert, deviceCert tls.Certificate
	var deviceCertPem []byte
	deviceCertSet := false

	if operations["selfRegister"] ||
		(operations["ping"] && forceOnboardingCert) {
		var err error
		onboardCert, err = tls.LoadX509KeyPair(onboardCertName, onboardKeyName)
		if err != nil {
			log.Fatal(err)
		}
		// Load device text cert for upload
		deviceCertPem, err = ioutil.ReadFile(deviceCertName)
		if err != nil {
			log.Fatal(err)
		}
	}
	if operations["getUuid"] ||
		(operations["ping"] && !forceOnboardingCert) {
		// Load device cert
		var err error
		deviceCert, err = tls.LoadX509KeyPair(deviceCertName,
			deviceKeyName)
		if err != nil {
			log.Fatal(err)
		}
		deviceCertSet = true
	}

	server, err := ioutil.ReadFile(serverFileName)
	if err != nil {
		log.Fatal(err)
	}
	serverNameAndPort := strings.TrimSpace(string(server))
	serverName := strings.Split(serverNameAndPort, ":")[0]
	// XXX for local testing
	// serverNameAndPort = "localhost:9069"

	// Post something without a return type.
	// Returns true when done; false when retry
	myPost := func(retryCount int, url string, reqlen int64, b *bytes.Buffer) bool {
		resp, contents, err := zedcloud.SendOnAllIntf(zedcloudCtx,
			serverNameAndPort+url, reqlen, b, retryCount, false)
		if err != nil {
			log.Errorln(err)
			return false
		}

		// Inform ledmanager about cloud connectivity
		types.UpdateLedManagerConfig(3)

		switch resp.StatusCode {
		case http.StatusOK:
			// Inform ledmanager about existence in cloud
			types.UpdateLedManagerConfig(4)
			log.Infof("%s StatusOK\n", url)
		case http.StatusCreated:
			// Inform ledmanager about existence in cloud
			types.UpdateLedManagerConfig(4)
			log.Infof("%s StatusCreated\n", url)
		case http.StatusConflict:
			// Inform ledmanager about brokenness
			types.UpdateLedManagerConfig(10)
			log.Errorf("%s StatusConflict\n", url)
			// Retry until fixed
			log.Errorf("%s\n", string(contents))
			return false
		case http.StatusNotModified: // XXX from zedcloud
			// Inform ledmanager about brokenness
			types.UpdateLedManagerConfig(10)
			log.Errorf("%s StatusNotModified\n", url)
			// Retry until fixed
			log.Errorf("%s\n", string(contents))
			return false
		default:
			log.Errorf("%s statuscode %d %s\n",
				url, resp.StatusCode,
				http.StatusText(resp.StatusCode))
			log.Errorf("%s\n", string(contents))
			return false
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			log.Errorf("%s no content-type\n", url)
			return false
		}
		mimeType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			log.Errorf("%s ParseMediaType failed %v\n", url, err)
			return false
		}
		switch mimeType {
		case "application/x-proto-binary", "application/json", "text/plain":
			log.Debugf("Received reply %s\n", string(contents))
		default:
			log.Errorln("Incorrect Content-Type " + mimeType)
			return false
		}
		return true
	}

	// Returns true when done; false when retry
	selfRegister := func(retryCount int) bool {
		tlsConfig, err := zedcloud.GetTlsConfig(serverName, &onboardCert)
		if err != nil {
			log.Errorln(err)
			return false
		}
		zedcloudCtx.TlsConfig = tlsConfig
		registerCreate := &zmet.ZRegisterMsg{
			PemCert: []byte(base64.StdEncoding.EncodeToString(deviceCertPem)),
		}
		b, err := proto.Marshal(registerCreate)
		if err != nil {
			log.Errorln(err)
			return false
		}
		return myPost(retryCount, "/api/v1/edgedevice/register",
			int64(len(b)), bytes.NewBuffer(b))
	}

	// Get something without a return type; used by ping
	// Returns true when done; false when retry.
	// Returns the response when done. Caller can not use resp.Body but
	// can use the contents []byte
	myGet := func(url string, retryCount int) (bool, *http.Response, []byte) {
		resp, contents, err := zedcloud.SendOnAllIntf(zedcloudCtx,
			serverNameAndPort+url, 0, nil, retryCount, false)
		if err != nil {
			log.Errorln(err)
			return false, nil, nil
		}

		switch resp.StatusCode {
		case http.StatusOK:
			log.Infof("%s StatusOK\n", url)
			return true, resp, contents
		default:
			log.Errorf("%s statuscode %d %s\n",
				url, resp.StatusCode,
				http.StatusText(resp.StatusCode))
			log.Errorf("Received %s\n", string(contents))
			return false, nil, nil
		}
	}

	// Setup HTTPS client for deviceCert unless force
	var cert tls.Certificate
	if forceOnboardingCert || operations["selfRegister"] {
		log.Infof("Using onboarding cert\n")
		cert = onboardCert
	} else if deviceCertSet {
		log.Infof("Using device cert\n")
		cert = deviceCert
	} else {
		log.Fatalf("No device certificate for %v\n", operations)
	}
	tlsConfig, err := zedcloud.GetTlsConfig(serverName, &cert)
	if err != nil {
		log.Fatal(err)
	}
	zedcloudCtx.TlsConfig = tlsConfig

	if operations["ping"] {
		url := "/api/v1/edgedevice/ping"
		retryCount := 0
		done := false
		var delay time.Duration
		for !done {
			time.Sleep(delay)
			done, _, _ = myGet(url, retryCount)
			if done {
				continue
			}
			retryCount += 1
			if maxRetries != 0 && retryCount > maxRetries {
				log.Infof("Exceeded %d retries for ping\n",
					maxRetries)
				os.Exit(1)
			}
			delay = 2 * (delay + time.Second)
			if delay > maxDelay {
				delay = maxDelay
			}
			log.Infof("Retrying ping in %d seconds\n",
				delay/time.Second)
		}
	}

	if operations["selfRegister"] {
		retryCount := 0
		done := false
		var delay time.Duration
		for !done {
			time.Sleep(delay)
			done = selfRegister(retryCount)
			if done {
				continue
			}
			retryCount += 1
			if maxRetries != 0 && retryCount > maxRetries {
				log.Errorf("Exceeded %d retries for selfRegister\n",
					maxRetries)
				os.Exit(1)
			}
			delay = 2 * (delay + time.Second)
			if delay > maxDelay {
				delay = maxDelay
			}
			log.Infof("Retrying selfRegister in %d seconds\n",
				delay/time.Second)
		}
	}

	if operations["getUuid"] {
		var devUUID uuid.UUID
		var hardwaremodel string

		doWrite := true
		url := "/api/v1/edgedevice/config"
		retryCount := 0
		done := false
		var delay time.Duration
		for !done {
			var resp *http.Response
			var contents []byte

			time.Sleep(delay)
			done, resp, contents = myGet(url, retryCount)
			if done {
				var err error

				devUUID, hardwaremodel, err = parseConfig(url, resp, contents)
				if err == nil {
					// Inform ledmanager about config received from cloud
					types.UpdateLedManagerConfig(4)
					continue
				}
				// Keep on trying until it parses
				done = false
				log.Errorf("Failed parsing uuid: %s\n",
					err)
				continue
			}
			if oldUUID != nilUUID && retryCount > 2 {
				log.Infof("Sticking with old UUID\n")
				devUUID = oldUUID
				done = true
				continue
			}

			retryCount += 1
			if maxRetries != 0 && retryCount > maxRetries {
				log.Errorf("Exceeded %d retries for getUuid\n",
					maxRetries)
				os.Exit(1)
			}
			delay = 2 * (delay + time.Second)
			if delay > maxDelay {
				delay = maxDelay
			}
			log.Infof("Retrying config in %d seconds\n",
				delay/time.Second)

		}
		if oldUUID != nilUUID {
			if oldUUID != devUUID {
				log.Infof("Replacing existing UUID %s\n",
					oldUUID.String())
			} else {
				log.Infof("No change to UUID %s\n",
					devUUID)
				doWrite = false
			}
		} else {
			log.Infof("Got config with UUID %s\n", devUUID)
		}

		if doWrite {
			b := []byte(fmt.Sprintf("%s\n", devUUID))
			err = ioutil.WriteFile(uuidFileName, b, 0644)
			if err != nil {
				log.Fatal("WriteFile", err, uuidFileName)
			}
			log.Debugf("Wrote UUID %s\n", devUUID)
		}
		doWrite = true
		if hardwaremodel != "" {
			if oldHardwaremodel != hardwaremodel {
				log.Infof("Replacing existing hardwaremodel %s\n",
					oldHardwaremodel)
			} else {
				log.Infof("No change to hardwaremodel %s\n",
					hardwaremodel)
				doWrite = false
			}
		} else {
			log.Infof("Got config with no hardwaremodel\n")
			doWrite = false
		}

		if doWrite {
			// Note that no CRLF
			b := []byte(hardwaremodel)
			err = ioutil.WriteFile(hardwaremodelFileName, b, 0644)
			if err != nil {
				log.Fatal("WriteFile", err,
					hardwaremodelFileName)
			}
			log.Debugf("Wrote hardwaremodel %s\n", hardwaremodel)
		}
	}

	err = pub.Publish("global", zedcloud.GetCloudMetrics())
	if err != nil {
		log.Errorln(err)
	}
}

func handleGlobalConfigModify(ctxArg interface{}, key string,
	statusArg interface{}) {

	ctx := ctxArg.(*devicenetwork.DeviceNetworkContext)
	if key != "global" {
		log.Debugf("handleGlobalConfigModify: ignoring %s\n", key)
		return
	}
	log.Infof("handleGlobalConfigModify for %s\n", key)
	debug, _ = agentlog.HandleGlobalConfig(ctx.SubGlobalConfig, agentName,
		debugOverride)
	log.Infof("handleGlobalConfigModify done for %s\n", key)
}

func handleGlobalConfigDelete(ctxArg interface{}, key string,
	statusArg interface{}) {

	ctx := ctxArg.(*devicenetwork.DeviceNetworkContext)
	if key != "global" {
		log.Debugf("handleGlobalConfigDelete: ignoring %s\n", key)
		return
	}
	log.Infof("handleGlobalConfigDelete for %s\n", key)
	debug, _ = agentlog.HandleGlobalConfig(ctx.SubGlobalConfig, agentName,
		debugOverride)
	log.Infof("handleGlobalConfigDelete done for %s\n", key)
}

func handleDNSModify(ctxArg interface{}, key string, statusArg interface{}) {

	status := cast.CastDeviceNetworkStatus(statusArg)
	ctx := ctxArg.(*clientContext)
	if key != "global" {
		log.Infof("handleDNSModify: ignoring %s\n", key)
		return
	}
	log.Infof("handleDNSModify for %s\n", key)
	if cmp.Equal(ctx.deviceNetworkStatus, status) {
		return
	}
	log.Infof("handleDNSModify: changed %v",
		cmp.Diff(ctx.deviceNetworkStatus, status))
	*ctx.deviceNetworkStatus = status
	newAddrCount := types.CountLocalAddrAnyNoLinkLocal(*ctx.deviceNetworkStatus)
	if newAddrCount != 0 && ctx.usableAddressCount == 0 {
		log.Infof("DeviceNetworkStatus from %d to %d addresses\n",
			newAddrCount, ctx.usableAddressCount)
	}
	ctx.usableAddressCount = newAddrCount
	log.Infof("handleDNSModify done for %s\n", key)
}

func handleDNSDelete(ctxArg interface{}, key string,
	statusArg interface{}) {

	log.Infof("handleDNSDelete for %s\n", key)
	ctx := ctxArg.(*clientContext)

	if key != "global" {
		log.Infof("handleDNSDelete: ignoring %s\n", key)
		return
	}
	*ctx.deviceNetworkStatus = types.DeviceNetworkStatus{}
	newAddrCount := types.CountLocalAddrAnyNoLinkLocal(*ctx.deviceNetworkStatus)
	ctx.usableAddressCount = newAddrCount
	log.Infof("handleDNSDelete done for %s\n", key)
}
