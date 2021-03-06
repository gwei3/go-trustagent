// +build linux

/*
 * Copyright (C) 2020 Intel Corporation
 * SPDX-License-Identifier: BSD-3-Clause
 */
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"intel/isecl/go-trust-agent/v2/config"
	"intel/isecl/go-trust-agent/v2/constants"
	"intel/isecl/go-trust-agent/v2/resource"
	"intel/isecl/go-trust-agent/v2/tasks"
	"intel/isecl/go-trust-agent/v2/util"
	commonExec "intel/isecl/lib/common/v2/exec"
	commLog "intel/isecl/lib/common/v2/log"
	"intel/isecl/lib/common/v2/log/message"
	"intel/isecl/lib/common/v2/validation"
	"intel/isecl/lib/platform-info/v2/platforminfo"
	"intel/isecl/lib/tpmprovider/v2"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"github.com/pkg/errors"
)

var log = commLog.GetDefaultLogger()
var secLog = commLog.GetSecurityLogger()

const (
	SYSTEMCTL_START   = "start"
	SYSTEMCTL_STOP    = "stop"
	SYSTEMCTL_STATUS  = "status"
	SYSTEMCTL_RESTART = "restart"
)

func printUsage() {

usage := `
Usage:

  tagent <command> [arguments]

Available Commands:

  help|-h|-help       Show this help message.
  setup [all] [task]  Run setup task.
  uninstall           Uninstall trust agent.
  version             Print build version info.
  start               Start the trust agent service.
  stop                Stop the trust agent service.
  status              Get the status of the trust agent service.

Setup command usage:  tagent setup [task]

Available Tasks for 'setup':

  [all] [/path/to/trustagent.env]           - Runs all setup tasks to provision the trust agent. 
                                              If path to trustagent.env not provided, settings are sourced from the environment.
                                                    Required environment variables [in env/trustagent.env]:
                                                       - AAS_API_URL=<url>                                 : AAS API URL
                                                       - CMS_BASE_URL=<url>                                : CMS API URL
                                                       - CMS_TLS_CERT_SHA384=<CMS TLS cert sha384 hash>    : to ensure that TA is communicating with the right CMS instance
                                                       - BEARER_TOKEN=<token>                              : for authenticating with CMS and VS
                                                       - MTWILSON_API_URL=<url>                            : VS API URL
                                                    Optional Environment variables:
                                                       - TA_ENABLE_CONSOLE_LOG=<true/false>                : When 'true', logs are redirected to stdout. Defaults to false.
                                                       - TA_SERVER_IDLE_TIMEOUT=<t seconds>                : Sets the trust agent service's idle timeout. Defaults to 10 seconds.
                                                       - TA_SERVER_MAX_HEADER_BYTES=<n bytes>              : Sets trust agent service's maximum header bytes.  Defaults to 1MB.
                                                       - TA_SERVER_READ_TIMEOUT=<t seconds>                : Sets trust agent service's read timeout.  Defaults to 30 seconds.
                                                       - TA_SERVER_READ_HEADER_TIMEOUT=<t seconds>         : Sets trust agent service's read header timeout.  Defaults to 30 seconds.
                                                       - TA_SERVER_WRITE_TIMEOUT=<t seconds>               : Sets trust agent service's write timeout.  Defaults to 10 seconds.
                                                       - SAN_LIST=<host1,host2.acme.com,...>               : CSV list that sets the value for SAN list in the TA TLS certificate.
                                                                                                             Defaults to "127.0.0.1,localhost".
                                                       - TA_TLS_CERT_CN=<Common Name>                      : Sets the value for Common Name in the TA TLS certificate.  Defaults to "Trust Agent TLS Certificate".
                                                       - TPM_OWNER_SECRET=<40 byte hex>                    : When provided, setup uses the 40 character hex string for the TPM
                                                                                                             owner password. Auto-generated when not provided.
                                                       - TPM_QUOTE_IPV4=Y/N                                : When 'Y', used the local system's ip address a salt when processing
                                                                                                             TPM quotes.  Defaults to 'N'.
                                                       - TRUSTAGENT_LOG_LEVEL=<trace|debug|info|error>     : Sets the verbosity level of logging. Defaults to 'info'.
                                                       - TRUSTAGENT_PORT=<portnum>                         : The port on which the trust agent service will listen.
                                                                                                             Defaults to 1443

  download-ca-cert                          - Fetches the latest CMS Root CA Certificates, overwriting existing files.
                                                    Required environment variables:
                                                       - CMS_BASE_URL=<url>                                : CMS API URL
                                                       - CMS_TLS_CERT_SHA384=<CMS TLS cert sha384 hash>    : to ensure that TA is communicating with the right CMS instance
        
  download-cert                             - Fetches a signed TLS Certificate from CMS, overwriting existing files.
                                                    Required environment variables:
                                                       - CMS_BASE_URL=<url>                                : CMS API URL
                                                       - BEARER_TOKEN=<token>                              : for authenticating with CMS and VS
                                                    Optional Environment variables:
                                                       - SAN_LIST=<host1,host2.acme.com,...>               : CSV list that sets the value for SAN list in the TA TLS certificate.
                                                                                                             Defaults to "127.0.0.1,localhost".
                                                       - TA_TLS_CERT_CN=<Common Name>                      : Sets the value for Common Name in the TA TLS certificate.
                                                                                                             Defaults to "Trust Agent TLS Certificate".

  update-certificates                       - Runs 'download-ca-cert' and 'download-cert'
                                                    Required environment variables:
                                                        - CMS_BASE_URL=<url>                                : CMS API URL
                                                        - CMS_TLS_CERT_SHA384=<CMS TLS cert sha384 hash>    : to ensure that TA is communicating with the right CMS instance
                                                        - BEARER_TOKEN=<token>                              : for authenticating with CMS
                                                    Optional Environment variables:
                                                        - SAN_LIST=<host1,host2.acme.com,...>               : CSV list that sets the value for SAN list in the TA TLS certificate.
                                                                                                              Defaults to "127.0.0.1,localhost".
                                                        - TA_TLS_CERT_CN=<Common Name>                      : Sets the value for Common Name in the TA TLS certificate.  Defaults to "Trust Agent TLS Certificate".

  provision-attestation                     - Runs setup tasks associated with HVS/TPM provisioning.
                                                    Required environment variables:
                                                        - MTWILSON_API_URL=<url>                            : VS API URL
                                                        - BEARER_TOKEN=<token>                              : for authenticating with VS
                                                    Optional environment variables:
                                                        - TPM_OWNER_SECRET=<40 byte hex>                    : When provided, setup uses the 40 character hex string for the TPM
                                                                                                              owner password. Auto-generated when not provided.
                                                        - TPM_QUOTE_IPV4='Y'/'N'                            : When 'Y', used the local system's ip address a salt when processing
                                                                                                              TPM quotes.  Defaults to 'N'.

  create-host                                 - Registers the trust agent with the verification service.
                                                    Required environment variables:
                                                        - MTWILSON_API_URL=<url>                            : VS API URL
                                                        - BEARER_TOKEN=<token>                              : for authenticating with VS
                                                    Optional environment variables:
                                                        - TPM_OWNER_SECRET=<40 byte hex>                    : When provided, setup uses the 40 character hex string for the TPM
                                                                                                              owner password. Auto-generated when not provided.
                                                        - TPM_QUOTE_IPV4='Y'/'N'                            : When 'Y', used the local system's ip address a salt when processing
                                                                                                              TPM quotes.  Defaults to 'N'.

  create-host-unique-flavor                 - Populates the verification service with the host unique flavor
                                                    Required environment variables:
                                                        - MTWILSON_API_URL=<url>                            : VS API URL
                                                        - BEARER_TOKEN=<token>                              : for authenticating with VS

  get-configured-manifest                   - Uses environment variables to pull application-integrity 
                                              manifests from the verification service.
                                                     Required environment variables:
                                                        - MTWILSON_API_URL=<url>                            : VS API URL
                                                        - BEARER_TOKEN=<token>                              : for authenticating with VS
                                                        - FLAVOR_UUIDS=<uuid1,uuid2,[...]>                  : CSV list of flavor UUIDs
                                                        - FLAVOR_LABELS=<flavorlabel1,flavorlabel2,[...]>   : CSV list of flavor labels                                                   
    `

	fmt.Println(usage)
}

func updatePlatformInfo() error {
	log.Trace("main:updatePlatformInfo() Entering")
	defer log.Trace("main:updatePlatformInfo() Leaving")
	// make sure the system-info directory exists
	_, err := os.Stat(constants.SystemInfoDir)
	if err != nil {
		return errors.Wrapf(err, "main:updatePlatformInfo() Error while checking the existence of %s", constants.SystemInfoDir)
	}

	// create the 'platform-info' file
	f, err := os.Create(constants.PlatformInfoFilePath)
	defer f.Close()
	if err != nil {
		return errors.Wrapf(err, "main:updatePlatformInfo() Error while creating %s", constants.PlatformInfoFilePath)
	}

	// collect the platform info
	secLog.Infof("%s main:updatePlatformInfo() Trying to fetch platform info", message.SU)
	platformInfo, err := platforminfo.GetPlatformInfo()
	if err != nil {
		return errors.Wrap(err, "main:updatePlatformInfo() Error while fetching platform info")
	}

	// serialize to json
	b, err := json.Marshal(platformInfo)
	if err != nil {
		return errors.Wrap(err, "main:updatePlatformInfo() Error while serializing platform info")
	}

	_, err = f.Write(b)
	if err != nil {
		return errors.Wrapf(err, "main:updatePlatformInfo() Error while writing into File: %s", constants.PlatformInfoFilePath)
	}

	log.Info("main:updatePlatformInfo() Successfully updated platform-info")
	return nil
}

func updateMeasureLog() error {
	log.Trace("main:updateMeasureLog() Entering")
	defer log.Trace("main:updateMeasureLog() Leaving")

	secLog.Infof("%s main:updateMeasureLog() Running %s using system administrative privileges", message.SU, constants.ModuleAnalysis)
	cmd := exec.Command(constants.ModuleAnalysis)
	cmd.Dir = constants.BinDir
	results, err := cmd.Output()
	if err != nil {
		return errors.Errorf("main:updateMeasureLog() module_analysis_sh error: %s", results)
	}

	log.Info("main:updateMeasureLog() Successfully updated measureLog.xml")
	return nil
}

func printVersion() {

	versionInfo, err := util.GetVersionInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr,"Error while getting version info: %v \n", err)
		os.Exit(1)
	}

	if len(os.Args) > 2 && os.Args[2] == "short" {
		fmt.Printf("%d.%d\n", versionInfo.Major, versionInfo.Minor)
	} else {
		fmt.Printf(versionInfo.VersionString)
	}
}

func uninstall() error {

	// stop/disable tagent service (if installed and running)
	//
	// systemctl status tagent will...
	// return 4 if not present on the system
	// return 3 if stopped
	// return 0 if running
	//
	// If not present, do nothing
	// if stopped, remove
	// if running, stop and remove
	_, _, err := commonExec.RunCommandWithTimeout(constants.ServiceStatusCommand, 5)
	if err == nil {
		// installed and running, stop and disable
		_, _, _ = commonExec.RunCommandWithTimeout(constants.ServiceStopCommand, 5)
		_, _, _ = commonExec.RunCommandWithTimeout(constants.ServiceDisableCommand, 5)
	} else {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus := exitError.Sys().(syscall.WaitStatus)
			if waitStatus.ExitStatus() == 3 {
				// stopped, just disable
				_, _, _ = commonExec.RunCommandWithTimeout(constants.ServiceDisableCommand, 5)
			} else if waitStatus.ExitStatus() == 4 {
				// do nothing if not installed
			} else {
				return errors.Errorf("main:uninstall() Service status returned unhandled error code %d", waitStatus.ExitStatus())
			}
		} else {
			return errors.Errorf("main:uninstall() An unhandled error occurred with the tagent service: %s", err)
		}
	}

	// always disable 'tagent_init.service' since it is not expected to be running (i.e. it's 
	// a 'oneshot' service)
	_, _, _ = commonExec.RunCommandWithTimeout(constants.ServiceDisableInitCommand, 5)

	fmt.Println("TrustAgent service removed successfully")

	//
	// uninstall tbootxml (if uninstall script is present)
	//
	if _, err := os.Stat(constants.UninstallTbootXmScript); err == nil {
		_, _, err = commonExec.RunCommandWithTimeout(constants.UninstallTbootXmScript, 15)
		if err != nil {
			return errors.Errorf("main:uninstall() An error occurred while uninstalling tboot: %s", err)
		}
	}

	fmt.Println("Application-Agent removed successfully")

	//
	// remove all of tagent files (in /opt/trustagent/)
	//
	if _, err := os.Stat(constants.InstallationDir); err == nil {
		err = os.RemoveAll(constants.InstallationDir)
		if err != nil {
			log.Errorf("main:uninstall() An error occurred removing the trustagent files: %s", err)
		}
	}

	//
	// remove all of tagent files (in /var/log/trustagent)
	//
	if _, err := os.Stat(constants.LogDir); err == nil {
		err = os.RemoveAll(constants.LogDir)
		if err != nil {
			log.Errorf("main:uninstall() An error occurred removing the trustagent log files: %s", err)
		}
	}

	fmt.Println("TrustAgent files removed successfully")

	return nil
}

func main() {

	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "Invalid arguments: %s\n", os.Args)
		printUsage()
		os.Exit(1)
	}

	if err := validation.ValidateStrings(os.Args); err != nil {
		secLog.WithError(err).Errorf("%s main:main() Invalid arguments", message.InvalidInputBadParam)
		fmt.Fprintln(os.Stderr, "Invalid arguments")
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.NewConfigFromYaml(constants.ConfigFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while parsing configuration file %v \n", err)
		os.Exit(1)
	}

	currentUser, _ := user.Current()

	cmd := os.Args[1]
	switch cmd {
	case "version":
		printVersion()
	case "init":

		//
		// The trust-agent service requires files like platform-info and eventLog.xml to be up to
		// date.  It also needs to run as the tagent user for security reasons.
		//
		// 'tagent init' is run as root (as configured in 'tagent_init.service') to generate
		// those files and own the files by tagent user.  The 'tagent.service' is configured
		// to 'Require' 'tagent_init.service' so that running 'systemctl start tagent' will 
		// always run 'tagent_init'.
		//
		if currentUser.Username != constants.RootUserName {
			fmt.Printf("'tagent start' must be run as root, not  user '%s'\n", currentUser.Username)
			os.Exit(1)
		}

		cfg.LogConfiguration(cfg.Logging.LogEnableStdout)

		err = updatePlatformInfo()
		if err != nil {
			log.Errorf("main:main() Error while creating platform-info: %s\n", err.Error())
		}

		err = updateMeasureLog()
		if err != nil {
			log.Errorf("main:main() Error While creating measureLog.xml: %s\n", err.Error())
		}

		tagentUser, err := user.Lookup(constants.TagentUserName)
		if err != nil {
			log.Errorf("main:main() Could not find user '%s'", constants.TagentUserName)
			os.Exit(1)
		}

		uid, err := strconv.ParseUint(tagentUser.Uid, 10, 32)
		if err != nil {
			log.Errorf("main:main() Could not parse tagent user uid '%s'", tagentUser.Uid)
			os.Exit(1)
		}
		
		gid, err := strconv.ParseUint(tagentUser.Gid, 10, 32)
		if err != nil {
			log.Errorf("main:main() Could not parse tagent user gid '%s'", tagentUser.Gid)
			os.Exit(1)
		}

		// take ownership of all of the files in /opt/trusagent before forking the
		// tagent service
		_ = filepath.Walk(constants.InstallationDir, func(fileName string, info os.FileInfo, err error) error {
			//log.Infof("Owning file %s", fileName)
			err = os.Chown(fileName, int(uid), int(gid))
			if err != nil {
				log.Errorf("main:main() Could not own file '%s'", fileName)
				return err
			}

			return nil
		})

		_ = filepath.Walk(constants.LogDir, func(fileName string, info os.FileInfo, err error) error {
			err = os.Chown(fileName, int(uid), int(gid))
			if err != nil {
					log.Errorf("main:main() Could not own file '%s'", fileName)
					return err
			}

			return nil
		})

		fmt.Println("tagent 'init' completed successful")

	case "startService":
		if currentUser.Username != constants.TagentUserName {
			fmt.Printf("'tagent startWebService' must be run as the 'tagent' user, not  user '%s'\n", currentUser.Username)
			os.Exit(1)
		}

		cfg.LogConfiguration(cfg.Logging.LogEnableStdout)

		// make sure the config is valid before starting the trust agent service
		err = cfg.Validate()
		if err != nil {
			log.Errorf("main:main() Error while validating the configuration file: %s", err)
			os.Exit(1)
		}

		tpmFactory, err := tpmprovider.NewTpmFactory()
		if err != nil {
			log.Errorf("main:main() Could not create the tpm factory %+v", err)
			os.Exit(1)
		}

		// create and start webservice
		service, err := resource.CreateTrustAgentService(cfg, tpmFactory)
		if err != nil {
			log.Errorf("main:main() Error while creating trustagent service %+v", err)
			os.Exit(1)
		}

		service.Start()

	case "start":
		cfg.LogConfiguration(cfg.Logging.LogEnableStdout)

		output, err := run_systemctl(SYSTEMCTL_START)
		if err != nil {
			fmt.Fprintln(os.Stderr, "An error occurred attempting to start the Trust Agent Service...")
			fmt.Fprintln(os.Stderr, output)
			os.Exit(1)
		}

		fmt.Println("Successfully started the Trust Agent Service")

	case "status":
		cfg.LogConfiguration(cfg.Logging.LogEnableStdout)

		// systemctl status returns an error code when the service is not running --
		// don't report an error, just show the results to the console in either case
		output, _ := run_systemctl(SYSTEMCTL_STATUS)
		fmt.Fprintln(os.Stdout, output)

	case "stop":
		cfg.LogConfiguration(cfg.Logging.LogEnableStdout)

		output, err := run_systemctl(SYSTEMCTL_STOP)
		if err != nil {
			fmt.Fprintln(os.Stderr, "An error occurred attempting to stop the Trust Agent Service...")
			fmt.Fprintln(os.Stderr, output)
			os.Exit(1)
		}

		fmt.Println("Successfully stopped the Trust Agent Service")

	case "setup":

		cfg.LogConfiguration(cfg.Logging.LogEnableStdout)
		// only apply env vars to config before starting 'setup' tasks

		if currentUser.Username != constants.RootUserName {
			log.Errorf("main:main() 'tagent setup' must be run as root, not  user '%s'\n", currentUser.Username)
			os.Exit(1)
		}

		var setupCommand string
		var flags []string
		if len(os.Args) > 2 {
			if strings.Contains(os.Args[2], "trustagent.env"){
				sourceEnvFile(os.Args[2])
				setupCommand = tasks.DefaultSetupCommand
			} else{
				setupCommand = os.Args[2]
				flags = os.Args[2:]
			}
		} else {
			setupCommand = tasks.DefaultSetupCommand
		}

		err = cfg.LoadEnvironmentVariables()
		if err != nil{
			log.WithError(err).Error("Error loading environment variables")
			fmt.Fprintf(os.Stderr, "Error loading environment variables\n %v \n\n", err)
		}

		registry, err := tasks.CreateTaskRegistry(cfg, flags)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while creating task registry \n Error: %s\n", err.Error())
			log.Errorf("main:main() Error while creating task registry %+v", err)
			os.Exit(1)
		}

		err = registry.RunCommand(setupCommand)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while running setup Command %s, \n Error: %s\n ", setupCommand, err.Error())
			log.Errorf("main:main() Error while running setup Command %s, %+v", setupCommand, err)
			os.Exit(1)
		}

	case "config":
		if len(os.Args) != 3 {
			fmt.Printf("'config' requires an additional parameter.\n")
		}

		cfg.PrintConfigSetting(os.Args[2])

	case "uninstall":
		err = uninstall()
		if err != nil {
			fmt.Fprintf(os.Stderr, "main:main() Error while running uninstalling trustagent %+v\n", err)
			os.Exit(1)
		}

	case "help":
		fallthrough
	case "-help":
		fallthrough
	case "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Invalid option: '%s'\n\n", cmd)
		printUsage()
	}
}

func sourceEnvFile(trustagentEnvFile string){
	fi, err := os.Stat(trustagentEnvFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s file does not exist", trustagentEnvFile)
		os.Exit(1)
	}

	fileSz := fi.Size()
	if fileSz == 0 || fileSz > constants.TrustAgentEnvMaxLength{
		fmt.Fprintf(os.Stderr, "%s file size exceeds maximum length: %d", trustagentEnvFile, constants.TrustAgentEnvMaxLength)
		os.Exit(1)
	}

    file, err := os.Open(trustagentEnvFile)
    if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open file: %s", trustagentEnvFile)
		os.Exit(1)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    var envKeyPair []string
    for scanner.Scan() {
		if scanner.Text() == "" || strings.HasPrefix("#", scanner.Text()) {
			continue
		}
		if strings.Contains(scanner.Text(), "="){
			envKeyPair = strings.Split(scanner.Text(), "=")
			os.Setenv(envKeyPair[0], envKeyPair[1]) 
		}
    }
}

func run_systemctl(systemCtlCmd string) (string, error) {
	log.Trace("main:run_systemctl() Entering")
	defer log.Trace("main:run_systemctl() Leaving")

	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error trying to look up for systemctl path")
		log.WithError(err).Error("main:run_systemctl() Error trying to look up for systemctl path")
		log.Tracef("%+v", err)
		os.Exit(1)
	}

	log.Infof("main:run_systemctl() Running 'systemctl %s tagent'", systemCtlCmd)

	cmd := exec.Command(systemctl, systemCtlCmd, "tagent")
	out, err := cmd.CombinedOutput()
	if err != nil && systemCtlCmd != SYSTEMCTL_STATUS {
		log.WithError(err).Errorf("main:run_systemctl() Error running 'systemctl %s tagent'", systemCtlCmd)
		log.Tracef("%+v", err)
		return string(out), err
	}

	return string(out), nil
}
