/*
Copyright 2022.

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

package controllers

import (
	"context"

	querocomv1alpha1 "github.com/quero-edu/loki-rule-operator/api/v1alpha1"
	"github.com/quero-edu/loki-rule-operator/internal/logger"
	"github.com/quero-edu/loki-rule-operator/pkg/k8sutils"
	"github.com/quero-edu/loki-rule-operator/pkg/lokirule"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// LokiRuleReconciler reconciles a LokiRule object
type LokiRuleReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Logger            logger.Logger
	LokiRulesPath     string
	LokiLabelSelector *metav1.LabelSelector
	LokiNamespace     string
}

func getLokiStatefulSet(
	client client.Client,
	labelSelector *metav1.LabelSelector,
	namespace string,
	logger logger.Logger,
) (*appsv1.StatefulSet, error) {
	statefulSet, err := k8sutils.GetStatefulSet(
		client,
		labelSelector,
		namespace,
		k8sutils.Options{Ctx: context.Background(), Logger: logger},
	)

	if err != nil {
		return nil, err
	}

	return statefulSet, nil
}

func handleByEventType(r *LokiRuleReconciler) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			deletedInstance := e.Object.(*querocomv1alpha1.LokiRule)

			r.Logger.Info(
				"Reconciling deleted LokiRule",
				"namespace",
				deletedInstance.Namespace,
				"name",
				deletedInstance.Name,
			)

			lokiStatefulset, err := getLokiStatefulSet(
				r.Client,
				r.LokiLabelSelector,
				r.LokiNamespace,
				r.Logger,
			)
			if err != nil {
				r.Logger.Error(err, "Failed to get Loki statefulSet")
			}

			configMapName := lokirule.GenerateConfigMapName(deletedInstance)

			r.Logger.Info("Unmounting configMap from loki statefulSet")
			err = k8sutils.UnmountConfigMap(
				r.Client,
				configMapName,
				lokiStatefulset,
				k8sutils.Options{Ctx: context.Background(), Logger: r.Logger},
			)

			if err != nil {
				r.Logger.Error(err, "Failed to unmount configMap from deployments")
			}

			r.Logger.Info("Deleting configMap")

			err = k8sutils.DeleteConfigMap(
				r.Client,
				configMapName,
				r.LokiNamespace,
				k8sutils.Options{Ctx: context.Background(), Logger: r.Logger},
			)
			if err != nil {
				r.Logger.Error(err, "Failed to delete configMap")
			}

			r.Logger.Info("deleted LokiRule reconciled")

			return false
		},
	}
}

func (r *LokiRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&querocomv1alpha1.LokiRule{}).
		WithEventFilter(handleByEventType(r)).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *LokiRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	options := k8sutils.Options{Ctx: ctx, Logger: r.Logger}

	r.Logger.Info("Reconciling LokiRule", "namespace", req.NamespacedName)

	instance := &querocomv1alpha1.LokiRule{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	labels := lokirule.GenerateLokiRuleLabels(instance)
	configMapName := lokirule.GenerateConfigMapName(instance)

	configMap, err := k8sutils.CreateOrUpdateConfigMap(
		r.Client,
		r.LokiNamespace,
		configMapName,
		instance.Spec.Data,
		labels,
		options,
	)
	if err != nil {
		r.Logger.Error(err, "Failed to ensure configMap exists")
		return reconcile.Result{}, err
	}

	lokiStatefulset, err := getLokiStatefulSet(
		r.Client,
		r.LokiLabelSelector,
		r.LokiNamespace,
		r.Logger,
	)
	if err != nil {
		r.Logger.Error(err, "Failed to get loki statefulSet")
		return reconcile.Result{}, err
	}

	err = k8sutils.MountConfigMap(
		r.Client,
		configMap,
		r.LokiRulesPath,
		lokiStatefulset,
		options,
	)
	if err != nil {
		r.Logger.Error(err, "ConfigMap not attached")
		return reconcile.Result{}, err
	}

	r.Logger.Info("LokiRule Reconciled")

	return ctrl.Result{}, nil
}
