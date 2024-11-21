/*
Copyright 2023.

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

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrastructurev1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	ilog "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/proto"
)

type ElementalHostGrpcReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	grpcServer proto.Server
}

func (r *ElementalHostGrpcReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		Watches(
			&infrastructurev1.ElementalHost{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				predicate.GenerationChangedPredicate{},
				predicate.AnnotationChangedPredicate{},
				predicate.LabelChangedPredicate{}),
		).
		Complete(r); err != nil {
		return fmt.Errorf("initializing ElementalHostGrpcReconciler builder: %w", err)
	}
	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elementalhosts,verbs=watch
func (r *ElementalHostGrpcReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	logger := log.FromContext(ctx).
		WithValues(ilog.KeyNamespace, req.Namespace).
		WithValues(ilog.KeyElementalHost, req.Name)
	logger.Info("Reconciling ElementalHostGrpc")

	r.grpcServer.SendHostUpdate(req.NamespacedName)
	return ctrl.Result{}, nil
}
