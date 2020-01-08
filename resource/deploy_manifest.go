/*
* Copyright (C) 2019 Intel Corporation
* SPDX-License-Identifier: BSD-3-Clause
 */
package resource

import (
	"encoding/xml"
	"intel/isecl/go-trust-agent/constants"
	"intel/isecl/go-trust-agent/vsclient"
	"intel/isecl/lib/common/validation"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Writes the manifest xml received to /opt/trustagent/var/manifest_{UUID}.xml.
func deployManifest() endpointHandler {
	return func(httpWriter http.ResponseWriter, httpRequest *http.Request) error {
		log.Trace("resource/deploy_manifest:deployManifest() Entering")
		defer log.Trace("resource/deploy_manifest:deployManifest() Leaving")

		log.Debugf("Request: %s", httpRequest.URL.Path)

		// receive a manifest from hvs in the request body
		manifestXml, err := ioutil.ReadAll(httpRequest.Body)
		if err != nil {
			log.Errorf("%s: Error reading manifest xml: %s", httpRequest.URL.Path, err)
			return &endpointError{Message: "Error reading manifest xml", StatusCode: http.StatusBadRequest}
		}

		// make sure the xml is well formed
		manifest := vsclient.Manifest{}
		err = xml.Unmarshal(manifestXml, &manifest)
		if err != nil {
			log.Errorf("%s: Invalid xml format: %s", httpRequest.URL.Path, err)
			return &endpointError{Message: "Error: Invalid xml format", StatusCode: http.StatusBadRequest}
		}

		err = validation.ValidateUUIDv4(manifest.UUID)
		if err != nil {
			log.Errorf("%s: Invalid uuid %s", httpRequest.URL.Path, err)
			return &endpointError{Message: "Error: Invalid uuid", StatusCode: http.StatusBadRequest}
		}

		if len(manifest.Label) == 0 {
			log.Errorf("%s: The manifest did not contain a label", httpRequest.URL.Path)
			return &endpointError{Message: "Error: The manifest did not contain a label", StatusCode: http.StatusBadRequest}
		}

		if strings.Contains(manifest.Label, vsclient.DEFAULT_APPLICATION_FLAVOR_PREFIX) ||
			strings.Contains(manifest.Label, vsclient.DEFAULT_WORKLOAD_FLAVOR_PREFIX) {
			log.Infof("%s: Default flavor's manifest (%s) is part of installation, no need to deploy default flavor's manifest", httpRequest.URL.Path, manifest.Label)
			return &endpointError{Message: " Default flavor's manifest (%s) is part of installation", StatusCode: http.StatusBadRequest}
		}

		// establish the name of the manifest file and write the file
		manifestFile := constants.VarDir + "manifest_" + manifest.UUID + ".xml"
		err = ioutil.WriteFile(manifestFile, manifestXml, 0600)
		if err != nil {
			log.Errorf("%s: Could not write manifest: %s", httpRequest.URL.Path, err)
			return &endpointError{Message: "Error processing request", StatusCode: http.StatusInternalServerError}
		}

		httpWriter.WriteHeader(http.StatusOK)
		return nil
	}
}
