/*
 * Copyright (C) 2019 Intel Corporation
 * SPDX-License-Identifier: BSD-3-Clause
 */
package vsclient

import (
	"intel/isecl/lib/clients"
	"intel/isecl/go-trust-agent/config"
	"intel/isecl/lib/clients/aas"
	"intel/isecl/go-trust-agent/constants"
	"intel/isecl/lib/clients"
	"net/http"
	"net/url"
)

type VSClientFactory interface {
	HostsClient() HostsClient
	FlavorsClient() FlavorsClient
	ManifestsClient() ManifestsClient
	TpmEndorsementsClient() TpmEndorsementsClient
	PrivacyCAClient() PrivacyCAClient
	CACertificatesClient() CACertificatesClient
}

type VSClientConfig struct {
	// BaseURL specifies the URL base for the HVS, for example https://hvs.server:8443/v2
	BaseURL string
}

func NewVSClientFactory(vsClientConfig *VSClientConfig) (VSClientFactory, error) {

	_, err := url.ParseRequestURI(vsClientConfig.BaseURL)
	if err != nil {
		return nil, err
	}

	defaultFactory := defaultVSClientFactory{vsClientConfig}
	return &defaultFactory, nil
}

//-------------------------------------------------------------------------------------------------
// Implementation
//-------------------------------------------------------------------------------------------------

type defaultVSClientFactory struct {
	cfg *VSClientConfig
}

func (vsClientFactory *defaultVSClientFactory) FlavorsClient() FlavorsClient {
	return &flavorsClientImpl{vsClientFactory.createHttpClient(), vsClientFactory.cfg}
}

func (vsClientFactory *defaultVSClientFactory) HostsClient() HostsClient {
	return &hostsClientImpl{vsClientFactory.createHttpClient(), vsClientFactory.cfg}
}

func (vsClientFactory *defaultVSClientFactory) ManifestsClient() ManifestsClient {
	return &manifestsClientImpl{vsClientFactory.createHttpClient(), vsClientFactory.cfg}
}

func (vsClientFactory *defaultVSClientFactory) TpmEndorsementsClient() TpmEndorsementsClient {
	return &tpmEndorsementsClientImpl{vsClientFactory.createHttpClient(), vsClientFactory.cfg}
}

func (vsClientFactory *defaultVSClientFactory) PrivacyCAClient() PrivacyCAClient {
	return &privacyCAClientImpl{vsClientFactory.createHttpClient(), vsClientFactory.cfg}
}

func (vsClientFactory *defaultVSClientFactory) CACertificatesClient() CACertificatesClient {
	return &caCertificatesClientImpl{vsClientFactory.createHttpClient(), vsClientFactory.cfg}
}

func (vsClientFactory *defaultVSClientFactory) createHttpClient() *http.Client {
	// Here we need to return a client which has validated the HVS TLS cert-chain
	client, err := clients.HTTPClientWithCADir(constants.TrustedCaCertsDir)

	if err != nil {
		return err
	}
	return &http.Client{Transport: client.Transport}
}
