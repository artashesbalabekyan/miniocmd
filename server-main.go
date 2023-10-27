/*
 * MinIO Cloud Storage, (C) 2015-2019 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package miniocmd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/minio/cli"
	"github.com/minio/minio/cmd/config"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/cmd/rest"
	"github.com/minio/minio/pkg/bucket/bandwidth"
	"github.com/minio/minio/pkg/certs"
	"github.com/minio/minio/pkg/env"
	"github.com/minio/minio/pkg/fips"
	"github.com/minio/minio/pkg/madmin"
	"github.com/minio/minio/pkg/sync/errgroup"
)

// ServerFlags - server command specific flags
var ServerFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "address",
		Value: ":" + GlobalMinioDefaultPort,
		Usage: "bind to a specific ADDRESS:PORT, ADDRESS can be an IP or hostname",
	},
}

var serverCmd = cli.Command{
	Name:  "server",
	Usage: "start object storage server",
	// Flags:  append(ServerFlags, GlobalFlags...),
	// Action: serverMain,
	CustomHelpTemplate: `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} {{if .VisibleFlags}}[FLAGS] {{end}}DIR1 [DIR2..]
  {{.HelpName}} {{if .VisibleFlags}}[FLAGS] {{end}}DIR{1...64}
  {{.HelpName}} {{if .VisibleFlags}}[FLAGS] {{end}}DIR{1...64} DIR{65...128}

DIR:
  DIR points to a directory on a filesystem. When you want to combine
  multiple drives into a single large system, pass one directory per
  filesystem separated by space. You may also use a '...' convention
  to abbreviate the directory arguments. Remote directories in a
  distributed setup are encoded as HTTP(s) URIs.
{{if .VisibleFlags}}
FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}{{end}}
EXAMPLES:
  1. Start minio server on "/home/shared" directory.
     {{.Prompt}} {{.HelpName}} /home/shared

  2. Start single node server with 64 local drives "/mnt/data1" to "/mnt/data64".
     {{.Prompt}} {{.HelpName}} /mnt/data{1...64}

  3. Start distributed minio server on an 32 node setup with 32 drives each, run following command on all the nodes
     {{.Prompt}} {{.EnvVarSetCommand}} MINIO_ROOT_USER{{.AssignmentOperator}}minio
     {{.Prompt}} {{.EnvVarSetCommand}} MINIO_ROOT_PASSWORD{{.AssignmentOperator}}miniostorage
     {{.Prompt}} {{.HelpName}} http://node{1...32}.example.com/mnt/export{1...32}

  4. Start distributed minio server in an expanded setup, run the following command on all the nodes
     {{.Prompt}} {{.EnvVarSetCommand}} MINIO_ROOT_USER{{.AssignmentOperator}}minio
     {{.Prompt}} {{.EnvVarSetCommand}} MINIO_ROOT_PASSWORD{{.AssignmentOperator}}miniostorage
     {{.Prompt}} {{.HelpName}} http://node{1...16}.example.com/mnt/export{1...32} \
            http://node{17...64}.example.com/mnt/export{1...64}
`,
}

func serverCmdArgs(ctx *cli.Context) []string {
	v := env.Get(config.EnvArgs, "")
	if v == "" {
		// Fall back to older ENV MINIO_ENDPOINTS
		v = env.Get(config.EnvEndpoints, "")
	}
	if v == "" {
		if !ctx.Args().Present() || ctx.Args().First() == "help" {
			cli.ShowCommandHelpAndExit(ctx, ctx.Command.Name, 1)
		}
		return ctx.Args()
	}
	return strings.Fields(v)
}

func serverHandleCmdArgs(ctx *cli.Context) {
	// Handle common command args.
	handleCommonCmdArgs(ctx)

	logger.FatalIf(CheckLocalServerAddr(globalCLIContext.Addr), "Unable to validate passed arguments")

	var err error
	var setupType SetupType

	// Check and load TLS certificates.
	globalPublicCerts, globalTLSCerts, globalIsTLS, err = getTLSConfig()
	logger.FatalIf(err, "Unable to load the TLS configuration")

	// Check and load Root CAs.
	globalRootCAs, err = certs.GetRootCAs(globalCertsCADir.Get())
	logger.FatalIf(err, "Failed to read root CAs (%v)", err)

	// Add the global public crts as part of global root CAs
	for _, publicCrt := range globalPublicCerts {
		globalRootCAs.AddCert(publicCrt)
	}

	// Register root CAs for remote ENVs
	env.RegisterGlobalCAs(globalRootCAs)

	globalMinioAddr = globalCLIContext.Addr

	globalMinioHost, globalMinioPort = mustSplitHostPort(globalMinioAddr)
	globalEndpoints, setupType, err = createServerEndpoints(globalCLIContext.Addr, serverCmdArgs(ctx)...)
	logger.FatalIf(err, "Invalid command line arguments")

	globalLocalNodeName = GetLocalPeer(globalEndpoints, globalMinioHost, globalMinioPort)

	globalRemoteEndpoints = make(map[string]Endpoint)
	for _, z := range globalEndpoints {
		for _, ep := range z.Endpoints {
			if ep.IsLocal {
				globalRemoteEndpoints[globalLocalNodeName] = ep
			} else {
				globalRemoteEndpoints[ep.Host] = ep
			}
		}
	}

	// allow transport to be HTTP/1.1 for proxying.
	globalProxyTransport = newCustomHTTPProxyTransport(&tls.Config{
		RootCAs:          globalRootCAs,
		CipherSuites:     fips.CipherSuitesTLS(),
		CurvePreferences: fips.EllipticCurvesTLS(),
	}, rest.DefaultTimeout)()
	globalProxyEndpoints = GetProxyEndpoints(globalEndpoints)
	globalInternodeTransport = newInternodeHTTPTransport(&tls.Config{
		RootCAs:          globalRootCAs,
		CipherSuites:     fips.CipherSuitesTLS(),
		CurvePreferences: fips.EllipticCurvesTLS(),
	}, rest.DefaultTimeout)()

	// On macOS, if a process already listens on LOCALIPADDR:PORT, net.Listen() falls back
	// to IPv6 address ie minio will start listening on IPv6 address whereas another
	// (non-)minio process is listening on IPv4 of given port.
	// To avoid this error situation we check for port availability.
	logger.FatalIf(checkPortAvailability(globalMinioHost, globalMinioPort), "Unable to start the server")

	globalIsErasure = (setupType == ErasureSetupType)
	globalIsDistErasure = (setupType == DistErasureSetupType)
	if globalIsDistErasure {
		globalIsErasure = true
	}
}

func serverHandleEnvVars() {
	// Handle common environment variables.
	handleCommonEnvVars()
}

var globalHealStateLK sync.RWMutex

func newAllSubsystems() {
	if globalIsErasure {
		globalHealStateLK.Lock()
		// New global heal state
		globalAllHealState = newHealState(true)
		globalBackgroundHealState = newHealState(false)
		globalHealStateLK.Unlock()
	}

	// Create new notification system and initialize notification targets
	globalNotificationSys = NewNotificationSys(globalEndpoints)

	// Create new bucket metadata system.
	if globalBucketMetadataSys == nil {
		globalBucketMetadataSys = NewBucketMetadataSys()
	} else {
		// Reinitialize safely when testing.
		globalBucketMetadataSys.Reset()
	}

	// Create the bucket bandwidth monitor
	globalBucketMonitor = bandwidth.NewMonitor(GlobalServiceDoneCh)

	// Create a new config system.
	globalConfigSys = NewConfigSys()

	// Create new IAM system.
	globalIAMSys = NewIAMSys()

	// Create new policy system.
	globalPolicySys = NewPolicySys()

	// Create new lifecycle system.
	globalLifecycleSys = NewLifecycleSys()

	// Create new bucket encryption subsystem
	globalBucketSSEConfigSys = NewBucketSSEConfigSys()

	// Create new bucket object lock subsystem
	globalBucketObjectLockSys = NewBucketObjectLockSys()

	// Create new bucket quota subsystem
	globalBucketQuotaSys = NewBucketQuotaSys()

	// Create new bucket versioning subsystem
	if globalBucketVersioningSys == nil {
		globalBucketVersioningSys = NewBucketVersioningSys()
	} else {
		globalBucketVersioningSys.Reset()
	}

	// Create new bucket replication subsytem
	globalBucketTargetSys = NewBucketTargetSys()
}

func configRetriableErrors(err error) bool {
	// Initializing sub-systems needs a retry mechanism for
	// the following reasons:
	//  - Read quorum is lost just after the initialization
	//    of the object layer.
	//  - Write quorum not met when upgrading configuration
	//    version is needed, migration is needed etc.
	rquorum := InsufficientReadQuorum{}
	wquorum := InsufficientWriteQuorum{}

	// One of these retriable errors shall be retried.
	return errors.Is(err, errDiskNotFound) ||
		errors.Is(err, errConfigNotFound) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, errErasureWriteQuorum) ||
		errors.Is(err, errErasureReadQuorum) ||
		errors.As(err, &rquorum) ||
		errors.As(err, &wquorum) ||
		isErrBucketNotFound(err) ||
		errors.Is(err, os.ErrDeadlineExceeded)
}

func initServer(ctx context.Context, newObject ObjectLayer) error {
	// Once the config is fully loaded, initialize the new object layer.
	setObjectLayer(newObject)

	// Make sure to hold lock for entire migration to avoid
	// such that only one server should migrate the entire config
	// at a given time, this big transaction lock ensures this
	// appropriately. This is also true for rotation of encrypted
	// content.
	txnLk := newObject.NewNSLock(minioMetaBucket, minioConfigPrefix+"/transaction.lock")

	// ****  WARNING ****
	// Migrating to encrypted backend should happen before initialization of any
	// sub-systems, make sure that we do not move the above codeblock elsewhere.

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	lockTimeout := newDynamicTimeout(5*time.Second, 3*time.Second)

	var err error
	for {
		select {
		case <-ctx.Done():
			// Retry was canceled successfully.
			return fmt.Errorf("Initializing sub-systems stopped gracefully %w", ctx.Err())
		default:
		}

		// let one of the server acquire the lock, if not let them timeout.
		// which shall be retried again by this loop.
		if _, err = txnLk.GetLock(ctx, lockTimeout); err != nil {
			logger.Info("Waiting for all MinIO sub-systems to be initialized.. trying to acquire lock")

			time.Sleep(time.Duration(r.Float64() * float64(5*time.Second)))
			continue
		}

		// These messages only meant primarily for distributed setup, so only log during distributed setup.
		if globalIsDistErasure {
			logger.Info("Waiting for all MinIO sub-systems to be initialized.. lock acquired")
		}

		// Migrate all backend configs to encrypted backend configs, optionally
		// handles rotating keys for encryption, if there is any retriable failure
		// that shall be retried if there is an error.
		if err = handleEncryptedConfigBackend(newObject); err == nil {
			// Upon success migrating the config, initialize all sub-systems
			// if all sub-systems initialized successfully return right away
			if err = initAllSubsystems(ctx, newObject); err == nil {
				txnLk.Unlock()
				// All successful return.
				if globalIsDistErasure {
					// These messages only meant primarily for distributed setup, so only log during distributed setup.
					logger.Info("All MinIO sub-systems initialized successfully")
				}
				return nil
			}
		}

		txnLk.Unlock() // Unlock the transaction lock and allow other nodes to acquire the lock if possible.

		if configRetriableErrors(err) {
			logger.Info("Waiting for all MinIO sub-systems to be initialized.. possible cause (%v)", err)
			time.Sleep(time.Duration(r.Float64() * float64(5*time.Second)))
			continue
		}

		// Any other unhandled return right here.
		return fmt.Errorf("Unable to initialize sub-systems: %w", err)
	}
}

func initAllSubsystems(ctx context.Context, newObject ObjectLayer) (err error) {
	// %w is used by all error returns here to make sure
	// we wrap the underlying error, make sure when you
	// are modifying this code that you do so, if and when
	// you want to add extra context to your error. This
	// ensures top level retry works accordingly.
	// List buckets to heal, and be re-used for loading configs.

	buckets, err := newObject.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("Unable to list buckets to heal: %w", err)
	}

	if globalIsErasure {
		if len(buckets) > 0 {
			if len(buckets) == 1 {
				logger.Info(fmt.Sprintf("Verifying if %d bucket is consistent across drives...", len(buckets)))
			} else {
				logger.Info(fmt.Sprintf("Verifying if %d buckets are consistent across drives...", len(buckets)))
			}
		}

		// Limit to no more than 50 concurrent buckets.
		g := errgroup.WithNErrs(len(buckets)).WithConcurrency(50)
		ctx, cancel := g.WithCancelOnError(ctx)
		defer cancel()
		for index := range buckets {
			index := index
			g.Go(func() error {
				_, berr := newObject.HealBucket(ctx, buckets[index].Name, madmin.HealOpts{Recreate: true})
				return berr
			}, index)
		}
		if err := g.WaitErr(); err != nil {
			return fmt.Errorf("Unable to list buckets to heal: %w", err)
		}
	}

	// Initialize config system.
	if err = globalConfigSys.Init(newObject); err != nil {
		if configRetriableErrors(err) {
			return fmt.Errorf("Unable to initialize config system: %w", err)
		}
		// Any other config errors we simply print a message and proceed forward.
		logger.LogIf(ctx, fmt.Errorf("Unable to initialize config, some features may be missing %w", err))
	}

	// Populate existing buckets to the etcd backend
	if globalDNSConfig != nil {
		// Background this operation.
		go initFederatorBackend(buckets, newObject)
	}

	// Initialize bucket metadata sub-system.
	globalBucketMetadataSys.Init(ctx, buckets, newObject)

	// Initialize notification system.
	globalNotificationSys.Init(ctx, buckets, newObject)

	// Initialize bucket targets sub-system.
	globalBucketTargetSys.Init(ctx, buckets, newObject)

	return nil
}

// Initialize object layer with the supplied disks, objectLayer is nil upon any error.
func newObjectLayer(ctx context.Context, endpointServerPools EndpointServerPools) (newObject ObjectLayer, err error) {
	// For FS only, directly use the disk.
	if endpointServerPools.NEndpoints() == 1 {
		// Initialize new FS object layer.
		return NewFSObjectLayer(endpointServerPools[0].Endpoints[0].Path)
	}

	return newErasureServerPools(ctx, endpointServerPools)
}
