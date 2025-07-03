/*
Copyright 2025 The Apache Software Foundation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/controllers"
	"github.com/veteran-chad/storm-controller/pkg/jarextractor"
	_ "github.com/veteran-chad/storm-controller/pkg/metrics" // Register metrics
	"github.com/veteran-chad/storm-controller/pkg/storm"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(stormv1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var stormClusterName string
	var stormNamespace string
	var nimbusHost string
	var nimbusPort int
	var uiHost string
	var uiPort int

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&stormClusterName, "storm-cluster", "storm-cluster", "Name of the StormCluster resource to manage")
	flag.StringVar(&stormNamespace, "storm-namespace", "default", "Namespace of the Storm cluster")
	flag.StringVar(&nimbusHost, "nimbus-host", "", "Nimbus host (defaults to {storm-cluster}-nimbus)")
	flag.IntVar(&nimbusPort, "nimbus-port", 6627, "Nimbus Thrift port")
	flag.StringVar(&uiHost, "ui-host", "", "Storm UI host (defaults to {storm-cluster}-ui)")
	flag.IntVar(&uiPort, "ui-port", 8080, "Storm UI port")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Set defaults for hosts if not provided
	if nimbusHost == "" {
		nimbusHost = stormClusterName + "-nimbus." + stormNamespace + ".svc.cluster.local"
	}
	if uiHost == "" {
		uiHost = stormClusterName + "-ui." + stormNamespace + ".svc.cluster.local"
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "storm-controller.apache.org",
		// Namespace scoped - controller only watches its own namespace
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				stormNamespace: {},
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create Storm client
	stormClient := storm.NewClient(nimbusHost, nimbusPort, uiHost, uiPort)

	// Setup StormCluster controller
	if err = (&controllers.StormClusterReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		StormClient: stormClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "StormCluster")
		os.Exit(1)
	}

	// Create JAR extractor
	jarExtractor := jarextractor.NewExtractor(mgr.GetClient(), stormNamespace)

	// Setup StormTopology controller
	if err = (&controllers.StormTopologyReconcilerSimple{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		StormClient:  stormClient,
		JarExtractor: jarExtractor,
		ClusterName:  stormClusterName,
		Namespace:    stormNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "StormTopology")
		os.Exit(1)
	}

	// Setup StormWorkerPool controller
	if err = (&controllers.StormWorkerPoolReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Namespace: stormNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "StormWorkerPool")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
