package amqstreams

import (
	"context"
	"fmt"

	"github.com/integr8ly/integreatly-operator/pkg/apis/integreatly/v1alpha1"
	kafkav1 "github.com/integr8ly/integreatly-operator/pkg/apis/kafka.strimzi.io/v1alpha1"
	"github.com/integr8ly/integreatly-operator/pkg/controller/installation/marketplace"
	"github.com/integr8ly/integreatly-operator/pkg/controller/installation/products/config"
	"github.com/integr8ly/integreatly-operator/pkg/resources"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/lib/ownerutil"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	defaultInstallationNamespace = "amq-streams"
	defaultSubscriptionName      = "amq-streams"
)

type Reconciler struct {
	Config        *config.AMQStreams
	ConfigManager config.ConfigReadWriter
	mpm           marketplace.MarketplaceInterface
	logger        *logrus.Entry
	*resources.Reconciler
}

func NewReconciler(configManager config.ConfigReadWriter, instance *v1alpha1.Installation, mpm marketplace.MarketplaceInterface) (*Reconciler, error) {
	config, err := configManager.ReadAMQStreams()
	if err != nil {
		return nil, errors.Wrap(err, "could not read nexus config")
	}

	if config.GetNamespace() == "" {
		config.SetNamespace(instance.Spec.NamespacePrefix + defaultInstallationNamespace)
	}

	logger := logrus.NewEntry(logrus.StandardLogger())

	return &Reconciler{
		ConfigManager: configManager,
		Config:        config,
		mpm:           mpm,
		logger:        logger,
		Reconciler:    resources.NewReconciler(mpm),
	}, nil
}

// Reconcile reads that state of the cluster for amq streams and makes changes based on the state read
// and what is required
func (r *Reconciler) Reconcile(ctx context.Context, inst *v1alpha1.Installation, serverClient pkgclient.Client) (v1alpha1.StatusPhase, error) {
	ns := r.Config.GetNamespace()

	phase, err := r.ReconcileNamespace(ctx, ns, inst, serverClient)
	if err != nil || phase != v1alpha1.PhaseCompleted {
		return phase, err
	}

	phase, err = r.ReconcileSubscription(ctx, inst, defaultSubscriptionName, ns, serverClient)
	if err != nil || phase != v1alpha1.PhaseCompleted {
		return phase, err
	}

	phase, err = r.handleCreatingComponents(ctx, serverClient, inst)
	if err != nil || phase != v1alpha1.PhaseCompleted {
		return phase, err
	}

	phase, err = r.handleProgressPhase(ctx, serverClient)
	if err != nil || phase != v1alpha1.PhaseCompleted {
		return phase, err
	}

	r.logger.Infof("%s has reconciled successfully", r.Config.GetProductName())
	return v1alpha1.PhaseCompleted, nil
}

func (r *Reconciler) handleCreatingComponents(ctx context.Context, client pkgclient.Client, inst *v1alpha1.Installation) (v1alpha1.StatusPhase, error) {
	r.logger.Debug("reconciling amq streams custom resource")

	kafka := &kafkav1.Kafka{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf(
				"%s/%s",
				kafkav1.SchemeGroupVersion.Group,
				kafkav1.SchemeGroupVersion.Version),
			Kind: kafkav1.KafkaKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "integreatly-cluster",
			Namespace: r.Config.GetNamespace(),
		},
		Spec: kafkav1.KafkaSpec{
			Kafka: kafkav1.KafkaSpecKafka{
				Version:  "2.1.1",
				Replicas: 3,
				Listeners: map[string]kafkav1.KafkaListener{
					"plain": {},
					"tls":   {},
				},
				Config: kafkav1.KafkaSpecKafkaConfig{
					OffsetsTopicReplicationFactor:        "3",
					TransactionStateLogReplicationFactor: "3",
					TransactionStateLogMinIsr:            "2",
					LogMessageFormatVersion:              "2.1",
				},
				Storage: kafkav1.KafkaStorage{
					Type:        "persistent-claim",
					Size:        "10Gi",
					DeleteClaim: false,
				},
			},
			Zookeeper: kafkav1.KafkaSpecZookeeper{
				Replicas: 3,
				Storage: kafkav1.KafkaStorage{
					Type:        "persistent-claim",
					Size:        "10Gi",
					DeleteClaim: false,
				},
			},
			EntityOperator: kafkav1.KafkaSpecEntityOperator{
				TopicOperator: kafkav1.KafkaTopicOperator{},
				UserOperator:  kafkav1.KafkaUserOperator{},
			},
		},
	}
	ownerutil.EnsureOwner(kafka, inst)

	// attempt to create the custom resource
	if err := client.Create(ctx, kafka); err != nil && !k8serr.IsAlreadyExists(err) {
		return v1alpha1.PhaseFailed, errors.Wrap(err, "failed to get or create a nexus custom resource")
	}

	// if there are no errors, the phase is complete
	return v1alpha1.PhaseCompleted, nil
}

func (r *Reconciler) handleProgressPhase(ctx context.Context, client pkgclient.Client) (v1alpha1.StatusPhase, error) {
	r.logger.Debug("checking amq streams pods are running")

	pods := &v1.PodList{}
	err := client.List(ctx, &pkgclient.ListOptions{Namespace: r.Config.GetNamespace()}, pods)
	if err != nil {
		return v1alpha1.PhaseFailed, errors.Wrap(err, "failed to check AMQ Streams installation")
	}

	//expecting 8 pods in total
	if len(pods.Items) < 8 {
		return v1alpha1.PhaseInProgress, nil
	}

	//and they should all be ready
checkPodStatus:
	for _, pod := range pods.Items {
		for _, cnd := range pod.Status.Conditions {
			if cnd.Type == v1.ContainersReady {
				if cnd.Status != v1.ConditionStatus("True") {
					return v1alpha1.PhaseInProgress, nil
				}
				break checkPodStatus
			}
		}
	}

	r.logger.Infof("all pods ready, returning complete")
	return v1alpha1.PhaseCompleted, nil
}
