package operator2

import (
	"context"
	"os"
	"time"

	kubemigratorclient "github.com/kubernetes-sigs/kube-storage-version-migrator/pkg/clients/clientset"
	migrationv1alpha1informer "github.com/kubernetes-sigs/kube-storage-version-migrator/pkg/clients/informer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	apiregistrationinformers "k8s.io/kube-aggregator/pkg/client/informers/externalversions"
	utilpointer "k8s.io/utils/pointer"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned"
	authopclient "github.com/openshift/client-go/operator/clientset/versioned"
	authopinformer "github.com/openshift/client-go/operator/informers/externalversions"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	routeinformer "github.com/openshift/client-go/route/informers/externalversions"
	"github.com/openshift/cluster-authentication-operator/pkg/controller/ingressstate"
	"github.com/openshift/cluster-authentication-operator/pkg/operator2/apiservices"
	"github.com/openshift/cluster-authentication-operator/pkg/operator2/assets"
	"github.com/openshift/cluster-authentication-operator/pkg/operator2/encryptionprovider"
	"github.com/openshift/cluster-authentication-operator/pkg/operator2/revisionclient"
	"github.com/openshift/cluster-authentication-operator/pkg/operator2/routercerts"
	"github.com/openshift/cluster-authentication-operator/pkg/operator2/workload"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	workloadcontroller "github.com/openshift/library-go/pkg/operator/apiserver/controller/workload"
	apiservercontrollerset "github.com/openshift/library-go/pkg/operator/apiserver/controllerset"
	"github.com/openshift/library-go/pkg/operator/encryption/controllers/migrators"
	encryptiondeployer "github.com/openshift/library-go/pkg/operator/encryption/deployer"
	"github.com/openshift/library-go/pkg/operator/loglevel"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/staleconditions"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/revision"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/unsupportedconfigoverridescontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	resync = 20 * time.Minute
)

// operatorContext holds combined data for both operators
type operatorContext struct {
	kubeClient     kubernetes.Interface
	configClient   configclient.Interface
	operatorClient *OperatorClient

	versionRecorder status.VersionGetter

	kubeInformersForNamespaces v1helpers.KubeInformersForNamespaces
	operatorConfigInformer     configinformer.SharedInformerFactory

	resourceSyncController *resourcesynccontroller.ResourceSyncController

	informersToRunFunc   []func(stopCh <-chan struct{})
	controllersToRunFunc []func(ctx context.Context, workers int)
}

// RunOperator prepares and runs both operators OAuth and OAuthAPIServer
// TODO: in the future we might move each operator to its onw pkg
// TODO: consider using the new operator framework
func RunOperator(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	kubeClient, err := kubernetes.NewForConfig(controllerContext.ProtoKubeConfig)
	if err != nil {
		return err
	}

	configClient, err := configclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	authConfigClient, err := authopclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	kubeInformersForNamespaces := v1helpers.NewKubeInformersForNamespaces(
		kubeClient,
		"openshift-authentication",
		"openshift-config",
		"openshift-config-managed",
		"openshift-oauth-apiserver",
		"openshift-authentication-operator",
		"", // an informer for non-namespaced resources
		"kube-system",
	)

	// short resync period as this drives the check frequency when checking the .well-known endpoint. 20 min is too slow for that.
	authOperatorConfigInformers := authopinformer.NewSharedInformerFactoryWithOptions(authConfigClient, time.Second*30,
		authopinformer.WithTweakListOptions(singleNameListOptions("cluster")),
	)

	operatorClient := &OperatorClient{
		authOperatorConfigInformers,
		authConfigClient.OperatorV1(),
	}

	resourceSyncer := resourcesynccontroller.NewResourceSyncController(
		operatorClient,
		kubeInformersForNamespaces,
		v1helpers.CachedSecretGetter(kubeClient.CoreV1(), kubeInformersForNamespaces),
		v1helpers.CachedConfigMapGetter(kubeClient.CoreV1(), kubeInformersForNamespaces),
		controllerContext.EventRecorder,
	)

	operatorCtx := &operatorContext{}
	operatorCtx.versionRecorder = status.NewVersionGetter()
	operatorCtx.kubeClient = kubeClient
	operatorCtx.configClient = configClient
	operatorCtx.kubeInformersForNamespaces = kubeInformersForNamespaces
	operatorCtx.resourceSyncController = resourceSyncer
	operatorCtx.operatorClient = operatorClient
	operatorCtx.operatorConfigInformer = configinformer.NewSharedInformerFactoryWithOptions(configClient, resync)

	if err := prepareOauthOperator(controllerContext, operatorCtx); err != nil {
		return err
	}
	if err := prepareOauthAPIServerOperator(ctx, controllerContext, operatorCtx); err != nil {
		return err
	}

	operatorCtx.informersToRunFunc = append(operatorCtx.informersToRunFunc, kubeInformersForNamespaces.Start, authOperatorConfigInformers.Start, operatorCtx.operatorConfigInformer.Start)
	operatorCtx.controllersToRunFunc = append(operatorCtx.controllersToRunFunc, resourceSyncer.Run)

	for _, informerToRunFn := range operatorCtx.informersToRunFunc {
		informerToRunFn(ctx.Done())
	}
	for _, controllerRunFn := range operatorCtx.controllersToRunFunc {
		go controllerRunFn(ctx, 1)
	}

	<-ctx.Done()
	return nil
}

func prepareOauthOperator(controllerContext *controllercmd.ControllerContext, operatorCtx *operatorContext) error {
	routeClient, err := routeclient.NewForConfig(controllerContext.ProtoKubeConfig)
	if err != nil {
		return err
	}

	// protobuf can be used with non custom resources
	oauthClient, err := oauthclient.NewForConfig(controllerContext.ProtoKubeConfig)
	if err != nil {
		return err
	}

	openshiftAuthenticationInformers := operatorCtx.kubeInformersForNamespaces.InformersFor("openshift-authentication")
	kubeSystemNamespaceInformers := operatorCtx.kubeInformersForNamespaces.InformersFor("kube-system")
	routeInformersNamespaced := routeinformer.NewSharedInformerFactoryWithOptions(routeClient, resync,
		routeinformer.WithNamespace("openshift-authentication"),
		routeinformer.WithTweakListOptions(singleNameListOptions("oauth-openshift")),
	)

	// add syncing for the OAuth metadata ConfigMap
	if err := operatorCtx.resourceSyncController.SyncConfigMap(
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-config-managed", Name: "oauth-openshift"},
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-authentication", Name: "v4-0-config-system-metadata"},
	); err != nil {
		return err
	}

	// add syncing for router certs for all cluster ingresses
	if err := operatorCtx.resourceSyncController.SyncSecret(
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-authentication", Name: "v4-0-config-system-router-certs"},
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-config-managed", Name: "router-certs"},
	); err != nil {
		return err
	}

	// add syncing for the console-config ConfigMap (indirect watch for changes)
	if err := operatorCtx.resourceSyncController.SyncConfigMap(
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-authentication", Name: "v4-0-config-system-console-config"},
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-config-managed", Name: "console-config"},
	); err != nil {
		return err
	}

	operator := NewAuthenticationOperator(
		*operatorCtx.operatorClient,
		oauthClient.OauthV1(),
		openshiftAuthenticationInformers,
		kubeSystemNamespaceInformers,
		operatorCtx.kubeClient,
		routeInformersNamespaced.Route().V1().Routes(),
		routeClient.RouteV1(),
		operatorCtx.operatorConfigInformer,
		operatorCtx.configClient,
		operatorCtx.versionRecorder,
		controllerContext.EventRecorder,
		operatorCtx.resourceSyncController,
	)

	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		"authentication",
		[]configv1.ObjectReference{
			{Group: operatorv1.GroupName, Resource: "authentications", Name: "cluster"},
			{Group: configv1.GroupName, Resource: "authentications", Name: "cluster"},
			{Group: configv1.GroupName, Resource: "infrastructures", Name: "cluster"},
			{Group: configv1.GroupName, Resource: "oauths", Name: "cluster"},
			{Group: routev1.GroupName, Resource: "routes", Name: "oauth-openshift", Namespace: "openshift-authentication"},
			{Resource: "services", Name: "oauth-openshift", Namespace: "openshift-authentication"},
			{Resource: "namespaces", Name: "openshift-config"},
			{Resource: "namespaces", Name: "openshift-config-managed"},
			{Resource: "namespaces", Name: "openshift-authentication"},
			{Resource: "namespaces", Name: "openshift-authentication-operator"},
			{Resource: "namespaces", Name: "openshift-ingress"},
			{Resource: "namespaces", Name: "openshift-oauth-apiserver"},
		},
		operatorCtx.configClient.ConfigV1(),
		operatorCtx.operatorConfigInformer.Config().V1().ClusterOperators(),
		operatorCtx.operatorClient,
		operatorCtx.versionRecorder,
		controllerContext.EventRecorder,
	)

	staleConditions := staleconditions.NewRemoveStaleConditionsController(
		[]string{
			// in 4.1.0 this was accidentally in the list.  This can be removed in 4.3.
			"Degraded",
		},
		operatorCtx.operatorClient,
		controllerContext.EventRecorder,
	)

	configOverridesController := unsupportedconfigoverridescontroller.NewUnsupportedConfigOverridesController(operatorCtx.operatorClient, controllerContext.EventRecorder)
	logLevelController := loglevel.NewClusterOperatorLoggingController(operatorCtx.operatorClient, controllerContext.EventRecorder)

	routerCertsController := routercerts.NewRouterCertsDomainValidationController(
		operatorCtx.operatorClient,
		controllerContext.EventRecorder,
		operatorCtx.operatorConfigInformer.Config().V1().Ingresses(),
		openshiftAuthenticationInformers.Core().V1().Secrets(),
		"openshift-authentication",
		"v4-0-config-system-router-certs",
		"oauth-openshift",
	)

	ingressStateController := ingressstate.NewIngressStateController(
		openshiftAuthenticationInformers,
		operatorCtx.kubeClient.CoreV1(),
		operatorCtx.kubeClient.CoreV1(),
		operatorCtx.operatorClient,
		"openshift-authentication",
		controllerContext.EventRecorder)

	// TODO remove this controller once we support Removed
	managementStateController := management.NewOperatorManagementStateController("authentication", operatorCtx.operatorClient, controllerContext.EventRecorder)
	management.SetOperatorNotRemovable()
	// TODO move to config observers
	// configobserver.NewConfigObserver(...)

	operatorCtx.informersToRunFunc = append(operatorCtx.informersToRunFunc, routeInformersNamespaced.Start, kubeSystemNamespaceInformers.Start)

	operatorCtx.controllersToRunFunc = append(
		operatorCtx.controllersToRunFunc,
		clusterOperatorStatus.Run,
		configOverridesController.Run,
		logLevelController.Run,
		routerCertsController.Run,
		managementStateController.Run,
		func(ctx context.Context, workers int) { staleConditions.Run(ctx, workers) },
		func(ctx context.Context, workers int) { ingressStateController.Run(workers, ctx.Done()) },
		func(ctx context.Context, _ int) { operator.Run(ctx.Done()) })

	return nil
}

func prepareOauthAPIServerOperator(ctx context.Context, controllerContext *controllercmd.ControllerContext, operatorCtx *operatorContext) error {
	eventRecorder := controllerContext.EventRecorder.ForComponent("oauth-apiserver")

	// add syncing for etcd certs for oauthapi-server
	if err := operatorCtx.resourceSyncController.SyncConfigMap(
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-oauth-apiserver", Name: "etcd-serving-ca"},
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-config", Name: "etcd-serving-ca"},
	); err != nil {
		return err
	}
	if err := operatorCtx.resourceSyncController.SyncSecret(
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-oauth-apiserver", Name: "etcd-client"},
		resourcesynccontroller.ResourceLocation{Namespace: "openshift-config", Name: "etcd-client"},
	); err != nil {
		return err
	}

	apiregistrationv1Client, err := apiregistrationclient.NewForConfig(controllerContext.ProtoKubeConfig)
	if err != nil {
		return err
	}
	apiregistrationInformers := apiregistrationinformers.NewSharedInformerFactory(apiregistrationv1Client, 10*time.Minute)

	nodeProvider := encryptiondeployer.NewDeploymentNodeProvider("openshift-oauth-apiserver", operatorCtx.kubeInformersForNamespaces)
	deployer, err := encryptiondeployer.NewRevisionLabelPodDeployer("revision", "openshift-oauth-apiserver", operatorCtx.kubeInformersForNamespaces, operatorCtx.resourceSyncController, operatorCtx.kubeClient.CoreV1(), operatorCtx.kubeClient.CoreV1(), nodeProvider)
	if err != nil {
		return err
	}
	migrationClient := kubemigratorclient.NewForConfigOrDie(controllerContext.KubeConfig)
	migrationInformer := migrationv1alpha1informer.NewSharedInformerFactory(migrationClient, time.Minute*30)
	migrator := migrators.NewKubeStorageVersionMigrator(migrationClient, migrationInformer.Migration().V1alpha1(), operatorCtx.kubeClient.Discovery())
	encryptionProvider := encryptionprovider.New(
		"openshift-oauth-apiserver",
		"openshift-config-managed",
		"encryption.apiserver.operator.openshift.io/managed-by",
		[]schema.GroupResource{
			{Group: "oauth.openshift.io", Resource: "oauthaccesstokens"},
			{Group: "oauth.openshift.io", Resource: "oauthauthorizetokens"},
		},
		operatorCtx.kubeInformersForNamespaces,
	)

	authAPIServerWorkload := workload.NewOAuthAPIServerWorkload(
		operatorCtx.operatorClient.Client,
		workloadcontroller.CountNodesFuncWrapper(operatorCtx.kubeInformersForNamespaces.InformersFor("").Core().V1().Nodes().Lister()),
		workloadcontroller.EnsureAtMostOnePodPerNode,
		"openshift-oauth-apiserver",
		os.Getenv("IMAGE_OAUTH_APISERVER"),
		os.Getenv("OPERATOR_IMAGE"),
		operatorCtx.kubeClient,
		eventRecorder,
		operatorCtx.versionRecorder)

	apiServerControllers, err := apiservercontrollerset.NewAPIServerControllerSet(
		operatorCtx.operatorClient,
		eventRecorder,
	).WithWorkloadController(
		"OAuthAPIServerController",
		"openshift-authentication-operator",
		"openshift-oauth-apiserver",
		os.Getenv("OPERATOR_IMAGE_VERSION"),
		"oauth",
		"APIServer",
		operatorCtx.kubeClient,
		authAPIServerWorkload,
		operatorCtx.configClient.ConfigV1().ClusterOperators(),
		operatorCtx.versionRecorder,
		operatorCtx.kubeInformersForNamespaces,
		operatorCtx.operatorClient.Informers.Operator().V1().Authentications().Informer(),
	).WithStaticResourcesController(
		"APIServerStaticResources",
		assets.Asset,
		[]string{
			"oauth-apiserver/ns.yaml",
			"oauth-apiserver/apiserver-clusterrolebinding.yaml",
			"oauth-apiserver/svc.yaml",
			"oauth-apiserver/sa.yaml",
			"oauth-apiserver/cm.yaml",
		},
		operatorCtx.kubeInformersForNamespaces,
		operatorCtx.kubeClient,
	).WithRevisionController(
		"openshift-oauth-apiserver",
		nil,
		[]revision.RevisionResource{{
			Name:     "encryption-config",
			Optional: true,
		}},
		operatorCtx.kubeInformersForNamespaces.InformersFor("openshift-oauth-apiserver"),
		revisionclient.New(operatorCtx.operatorClient, operatorCtx.operatorClient.Client),
		v1helpers.CachedConfigMapGetter(operatorCtx.kubeClient.CoreV1(), operatorCtx.kubeInformersForNamespaces),
		v1helpers.CachedSecretGetter(operatorCtx.kubeClient.CoreV1(), operatorCtx.kubeInformersForNamespaces),
	).WithAPIServiceController(
		"openshift-apiserver",
		apiservices.NewAPIServicesToManage(
			operatorCtx.operatorClient.Informers.Operator().V1().Authentications().Lister(),
			func() []*apiregistrationv1.APIService {
				var apiServiceGroupVersions = []schema.GroupVersion{
					// these are all the apigroups we manage
					{Group: "oauth.openshift.io", Version: "v1"},
					{Group: "user.openshift.io", Version: "v1"},
				}

				ret := []*apiregistrationv1.APIService{}
				for _, apiServiceGroupVersion := range apiServiceGroupVersions {
					obj := &apiregistrationv1.APIService{
						ObjectMeta: metav1.ObjectMeta{
							Name: apiServiceGroupVersion.Version + "." + apiServiceGroupVersion.Group,
							Annotations: map[string]string{
								"service.alpha.openshift.io/inject-cabundle":   "true",
								"authentication.operator.openshift.io/managed": "true",
							},
						},
						Spec: apiregistrationv1.APIServiceSpec{
							Group:   apiServiceGroupVersion.Group,
							Version: apiServiceGroupVersion.Version,
							Service: &apiregistrationv1.ServiceReference{
								Namespace: "openshift-oauth-apiserver",
								Name:      "api",
								Port:      utilpointer.Int32Ptr(443),
							},
							GroupPriorityMinimum: 9900,
							VersionPriority:      15,
						},
					}
					ret = append(ret, obj)
				}

				return ret
			}(),
			eventRecorder,
		).GetAPIServicesToManage,
		apiregistrationInformers,
		apiregistrationv1Client.ApiregistrationV1(),
		operatorCtx.kubeInformersForNamespaces.InformersFor("openshift-oauth-apiserver"),
		operatorCtx.kubeClient,
	).WithEncryptionControllers(
		"openshift-oauth-apiserver",
		encryptionProvider,
		deployer,
		migrator,
		operatorCtx.kubeClient.CoreV1(),
		operatorCtx.configClient.ConfigV1().APIServers(),
		operatorCtx.operatorConfigInformer.Config().V1().APIServers(),
		operatorCtx.kubeInformersForNamespaces,
	).
		WithoutClusterOperatorStatusController().
		WithoutFinalizerController().
		WithoutLogLevelController().
		WithoutConfigUpgradableController().
		PrepareRun()

	if err != nil {
		return err
	}

	manageOAuthAPIController := apiservices.NewManageAPIServicesController(
		"MangeOAuthAPIController",
		operatorCtx.operatorClient.Client,
		operatorCtx.operatorClient.Informers,
		eventRecorder)

	operatorCtx.controllersToRunFunc = append(operatorCtx.controllersToRunFunc, func(ctx context.Context, _ int) { apiServerControllers.Run(ctx) }, manageOAuthAPIController.Run)
	operatorCtx.informersToRunFunc = append(operatorCtx.informersToRunFunc, apiregistrationInformers.Start, migrationInformer.Start)
	return nil
}

func singleNameListOptions(name string) func(opts *metav1.ListOptions) {
	return func(opts *metav1.ListOptions) {
		opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", name).String()
	}
}
