package bootstrap

import (
	"context"
	"fmt"
	"github.com/integr8ly/integreatly-operator/pkg/apis/integreatly/v1alpha1"
	"github.com/integr8ly/integreatly-operator/pkg/providers"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const(
	storageName = "integreatly-%s"
)

type Reconciler struct {
	cloudProvider providers.CloudServiceProvider
}

func NewReconciler(provider providers.CloudServiceProvider)*Reconciler  {
	return &Reconciler{
		cloudProvider:provider,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, in *v1alpha1.Installation, product *v1alpha1.InstallationProductStatus, serverClient pkgclient.Client) (v1alpha1.StatusPhase, error) {
	if in.Spec.Type == string(v1alpha1.InstallationTypeWorkshop){
		return v1alpha1.PhaseCompleted, nil
	}
	return "", nil
}

func (r *Reconciler)setupStorage(in *v1alpha1.InstallationProductStatus)(v1alpha1.StatusPhase, error)  {
	if err := r.cloudProvider.CreateCloudStorage(fmt.Sprintf(storageName, in.Name)); err != nil{
		if 	in.Status == "" || in.Status == v1alpha1.PhaseFailed{
			return v1alpha1.PhaseFailed, err
		}else if ! providers.IsAlreadyExistsErr(err){
			return v1alpha1.PhaseFailed, err
		}
	}
	return v1alpha1.PhaseCompleted, nil
}
