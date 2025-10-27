package omdemo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	applyoperatorv1 "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

type demoController struct {
	controllerInstanceName string
	operatorClient         v1helpers.OperatorClient
	kubeClient             kubernetes.Interface
	kubeConfigMapLister    corev1listers.ConfigMapLister
}

func NewDemoController(
	instanceName string,
	operatorClient v1helpers.OperatorClient,
	kubeClient kubernetes.Interface,
	kubeConfigMapInformer corev1informers.ConfigMapInformer,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &demoController{
		controllerInstanceName: factory.ControllerInstanceName(instanceName, "Demo"),
		operatorClient:         operatorClient,
		kubeClient:             kubeClient,
		kubeConfigMapLister:    kubeConfigMapInformer.Lister(),
	}
	return factory.New().
		WithSync(c.Sync).
		WithInformers(kubeConfigMapInformer.Informer()).
		ResyncEvery(time.Minute).
		WithControllerInstanceName(c.controllerInstanceName).
		ToController(
			"Demo",
			eventRecorder.WithComponentSuffix(c.controllerInstanceName),
		)
}

func (c *demoController) Sync(ctx context.Context, fCtx factory.SyncContext) error {
	time.Sleep(1 * time.Second)
	counter, err := c.syncInternal(ctx, fCtx)

	cond := applyoperatorv1.OperatorCondition().
		WithType("OpenshiftManagerDemo").
		WithStatus(operatorv1.ConditionFalse).
		WithReason("AllIsGood").
		WithMessage(fmt.Sprintf("Count:%d", counter))

	if err != nil {
		cond = cond.
			WithStatus(operatorv1.ConditionFalse).
			WithReason("Error").
			WithMessage(err.Error())
	}

	status := applyoperatorv1.OperatorStatus().WithConditions(cond)
	if statusErr := c.operatorClient.ApplyOperatorStatus(ctx, c.controllerInstanceName, status); statusErr != nil {
		klog.Errorf("failed to update operator status: %v", statusErr)
		err = statusErr
	}
	return err
}

func (c *demoController) syncInternal(ctx context.Context, _ factory.SyncContext) (int, error) {
	klog.Info("Sync called")
	defer klog.Info(" Sync ended")

	configMap, err := c.kubeConfigMapLister.ConfigMaps("openshift-authentication").Get("foo")
	if apierrors.IsNotFound(err) {
		configMap = makeConfigMap("foo")
		klog.Infof("Creating %s configmap in %s namspace because it was missing", configMap.Name, configMap.Namespace)
		_, err = c.kubeClient.CoreV1().ConfigMaps("openshift-authentication").Create(ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("Failed to create %s configmap in %s namespace: %v", configMap.Name, configMap.Namespace, err)
		}
		return 0, err
	}
	counterStr := configMap.Data["counter"]
	counter, err := strconv.Atoi(counterStr)
	if err != nil {
		return 0, err
	}
	counter = counter + 1
	configMap.Data["counter"] = fmt.Sprintf("%d", counter)
	klog.Infof("Updating the sync counter to %d for %s configmap in %s namspace", counter, configMap.Name, configMap.Namespace)
	_, err = c.kubeClient.CoreV1().ConfigMaps("openshift-authentication").Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update configmap %s/%s, err: %v", configMap.Name, configMap.Namespace, err)
	}
	return counter, err
}

func makeConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "openshift-authentication",
		},
		Data: map[string]string{"counter": "1"},
	}
}
