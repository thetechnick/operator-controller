package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	ocv1alpha1 "github.com/operator-framework/operator-controller/api/v1alpha1"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	pkocorev1alpha1 "package-operator.run/apis/core/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Uses Package Operator to reconcile a bundle on the cluster and get status information.
type pkoReconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *pkoReconciler) Reconcile(ctx context.Context, cext *ocv1alpha1.ClusterExtension, bundle *declcfg.Bundle) (
	res ctrl.Result, err error,
) {
	configJson, err := json.Marshal(map[string]interface{}{
		"namespace": cext.Spec.InstallNamespace,
	})
	if err != nil {
		return res, err
	}
	desiredPkg := &pkocorev1alpha1.ClusterPackage{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "package-operator.run/v1alpha1",
			Kind:       "ClusterPackage",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cext.Name,
			Labels: map[string]string{
				"olm.operatorframework.io/owner-kind": "ClusterExtension",
			},
		},
		Spec: pkocorev1alpha1.PackageSpec{
			Image: bundle.Image,
			Config: &runtime.RawExtension{
				Raw: configJson,
			},
		},
	}
	if err := controllerutil.SetControllerReference(cext, desiredPkg, r.scheme); err != nil {
		return res, fmt.Errorf("set controller ref: %w", err)
	}
	if err := r.client.Patch(ctx, desiredPkg, client.Apply, client.FieldOwner("olm-operator-controller")); err != nil {
		return res, fmt.Errorf("patching ClusterPackage: %w", err)
	}

	// Status reporting
	currentPkg := &pkocorev1alpha1.ClusterPackage{}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(desiredPkg), currentPkg); err != nil {
		return res, fmt.Errorf("getting ClusterPackage: %w", err)
	}

	availableCond := meta.FindStatusCondition(currentPkg.Status.Conditions, pkocorev1alpha1.PackageAvailable)
	if availableCond != nil && availableCond.ObservedGeneration == currentPkg.Generation {
		meta.SetStatusCondition(&cext.Status.Conditions, metav1.Condition{
			Type:               "package-operator.run/Available",
			Status:             availableCond.Status,
			Reason:             availableCond.Reason,
			Message:            availableCond.Message,
			ObservedGeneration: currentPkg.Generation,
		})
	}

	progressingCond := meta.FindStatusCondition(currentPkg.Status.Conditions, pkocorev1alpha1.PackageProgressing)
	if progressingCond != nil && progressingCond.ObservedGeneration == currentPkg.Generation {
		meta.SetStatusCondition(&cext.Status.Conditions, metav1.Condition{
			Type:               "package-operator.run/Progressing",
			Status:             progressingCond.Status,
			Reason:             progressingCond.Reason,
			Message:            progressingCond.Message,
			ObservedGeneration: currentPkg.Generation,
		})
	}

	unpackedCond := meta.FindStatusCondition(currentPkg.Status.Conditions, pkocorev1alpha1.PackageUnpacked)
	if unpackedCond != nil && unpackedCond.ObservedGeneration == currentPkg.Generation {
		meta.SetStatusCondition(&cext.Status.Conditions, metav1.Condition{
			Type:               "package-operator.run/Unpacked",
			Status:             unpackedCond.Status,
			Reason:             unpackedCond.Reason,
			Message:            unpackedCond.Message,
			ObservedGeneration: currentPkg.Generation,
		})
	}
	return
}
