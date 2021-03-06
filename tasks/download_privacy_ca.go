/*
 * Copyright (C) 2020 Intel Corporation
 * SPDX-License-Identifier: BSD-3-Clause
 */
package tasks

import (
	"fmt"
	"intel/isecl/go-trust-agent/v2/constants"
	"intel/isecl/go-trust-agent/v2/util"
	"intel/isecl/go-trust-agent/v2/vsclient"
	"intel/isecl/lib/common/v2/setup"
	"io/ioutil"

	"github.com/pkg/errors"
)

type DownloadPrivacyCA struct {
	clientFactory   vsclient.VSClientFactory
}

// Download's the privacy CA from HVS.
func (task *DownloadPrivacyCA) Run(c setup.Context) error {
	log.Trace("tasks/download_privacy_ca:Run() Entering")
	defer log.Trace("tasks/download_privacy_ca:Run() Leaving")
	fmt.Println("Running setup task: download-privacy-ca")

	privacyCAClient, err := task.clientFactory.PrivacyCAClient()
	if err != nil {
		log.WithError(err).Error("tasks/download_privacy_ca:Run() Could not create privacy-ca client")
		return err
	}

	ca, err := privacyCAClient.DownloadPrivacyCa()
	if err != nil {
		log.WithError(err).Error("tasks/download_privacy_ca:Run() Error while downloading privacyCA file")
		return errors.New("Error while downloading privacyCA file")
	}

	err = ioutil.WriteFile(constants.PrivacyCA, ca, 0644)
	if err != nil {
		log.WithError(err).Errorf("tasks/download_privacy_ca:Run() Error while writing privacy ca file '%s'", constants.PrivacyCA)
		return errors.Errorf("Error while writing privacy ca file '%s'", constants.PrivacyCA)
	}

	return nil
}

func (task *DownloadPrivacyCA) Validate(c setup.Context) error {
	log.Trace("tasks/download_privacy_ca:Validate() Entering")
	defer log.Trace("tasks/download_privacy_ca:Validate() Leaving")
	_, err := util.GetPrivacyCA()
	if err != nil {
		return err
	}

	log.Info("tasks/download_privacy_ca:Validate() Download PrivacyCA was successful")
	return nil
}
