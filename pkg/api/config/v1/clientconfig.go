/*
Copyright 2021 Adobe. All rights reserved.
This file is licensed to you under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License. You may obtain a copy
of the License at http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under
the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR REPRESENTATIONS
OF ANY KIND, either express or implied. See the License for the specific language
governing permissions and limitations under the License.
*/

package v1

import (
	"bytes"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"os"
	"path/filepath"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// fromFile provides an alternative to the deprecated ctrl.ConfigFile().AtPath(path).OfKind(&cfg)
func fromFile(path string, scheme *runtime.Scheme, cfg *ClientConfig) error {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}

	codecs := serializer.NewCodecFactory(scheme)

	// Regardless of if the bytes are of any external version,
	// it will be read successfully and converted into the internal version
	return runtime.DecodeInto(codecs.UniversalDecoder(), content, cfg)
}

// addTo provides an alternative to the deprecated o.AndFrom(&cfg)
func addTo(o *ctrl.Options, cfg *ClientConfig) {
	addLeaderElectionTo(o, cfg)
	if o.MetricsBindAddress == "" && cfg.Metrics.BindAddress != "" {
		o.MetricsBindAddress = cfg.Metrics.BindAddress
	}

	if o.HealthProbeBindAddress == "" && cfg.Health.HealthProbeBindAddress != "" {
		o.HealthProbeBindAddress = cfg.Health.HealthProbeBindAddress
	}

	if o.ReadinessEndpointName == "" && cfg.Health.ReadinessEndpointName != "" {
		o.ReadinessEndpointName = cfg.Health.ReadinessEndpointName
	}

	if o.LivenessEndpointName == "" && cfg.Health.LivenessEndpointName != "" {
		o.LivenessEndpointName = cfg.Health.LivenessEndpointName
	}
	addWebhookTo(o, cfg)
	if cfg.Controller != nil {
		if o.Controller.CacheSyncTimeout == 0 && cfg.Controller.CacheSyncTimeout != nil {
			o.Controller.CacheSyncTimeout = *cfg.Controller.CacheSyncTimeout
		}

		if len(o.Controller.GroupKindConcurrency) == 0 && len(cfg.Controller.GroupKindConcurrency) > 0 {
			o.Controller.GroupKindConcurrency = cfg.Controller.GroupKindConcurrency
		}
	}
}

func addLeaderElectionTo(o *ctrl.Options, cfg *ClientConfig) {
	if cfg.LeaderElection == nil {
		// The source does not have any ClientConfig; noop
		return
	}

	if !o.LeaderElection && cfg.LeaderElection.LeaderElect != nil {
		o.LeaderElection = *cfg.LeaderElection.LeaderElect
	}

	if o.LeaderElectionResourceLock == "" && cfg.LeaderElection.ResourceLock != "" {
		o.LeaderElectionResourceLock = cfg.LeaderElection.ResourceLock
	}

	if o.LeaderElectionNamespace == "" && cfg.LeaderElection.ResourceNamespace != "" {
		o.LeaderElectionNamespace = cfg.LeaderElection.ResourceNamespace
	}

	if o.LeaderElectionID == "" && cfg.LeaderElection.ResourceName != "" {
		o.LeaderElectionID = cfg.LeaderElection.ResourceName
	}

	if o.LeaseDuration == nil && !reflect.DeepEqual(cfg.LeaderElection.LeaseDuration, metav1.Duration{}) {
		o.LeaseDuration = &cfg.LeaderElection.LeaseDuration.Duration
	}

	if o.RenewDeadline == nil && !reflect.DeepEqual(cfg.LeaderElection.RenewDeadline, metav1.Duration{}) {
		o.RenewDeadline = &cfg.LeaderElection.RenewDeadline.Duration
	}

	if o.RetryPeriod == nil && !reflect.DeepEqual(cfg.LeaderElection.RetryPeriod, metav1.Duration{}) {
		o.RetryPeriod = &cfg.LeaderElection.RetryPeriod.Duration
	}
}

func addWebhookTo(o *ctrl.Options, cfg *ClientConfig) {
	if o.WebhookServer == nil && cfg.Webhook.Host != "" && *cfg.Webhook.Port > 0 && cfg.Webhook.CertDir != "" {
		o.WebhookServer = webhook.NewServer(webhook.Options{
			Host:    cfg.Webhook.Host,
			Port:    *cfg.Webhook.Port,
			CertDir: cfg.Webhook.CertDir,
		})
	}
}

// Encode returns a string representation of the given ClientConfig.
func Encode(scheme *runtime.Scheme, cfg *ClientConfig) (string, error) {
	codecs := serializer.NewCodecFactory(scheme)
	const mediaType = runtime.ContentTypeYAML
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), mediaType)
	if !ok {
		return "", fmt.Errorf("unable to locate encoder -- %q is not a supported media type", mediaType)
	}

	encoder := codecs.EncoderForVersion(info.Serializer, GroupVersion)
	buf := new(bytes.Buffer)
	if err := encoder.Encode(cfg, buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Load returns a set of controller options and ClientConfig from the given file, if the config file path is empty
// it uses the default values.
func Load(scheme *runtime.Scheme, configFile string, defaults *ClientConfig) (ctrl.Options, ClientConfig, error) {
	var err error
	options := ctrl.Options{
		Scheme: scheme,
	}
	cfg := &ClientConfig{}
	if defaults != nil {
		cfg = defaults.DeepCopy()
	}
	if configFile == "" {
		scheme.Default(cfg)
	} else {
		err := fromFile(configFile, scheme, cfg)
		if err != nil {
			return options, *cfg, err
		}
	}
	addTo(&options, cfg)
	return options, *cfg, err
}
