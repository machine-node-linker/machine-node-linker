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

	machinev1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=my.domain,resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=my.domain,resources=nodes/status,verbs=get
//+kubebuilder:rbac:groups=my.domain,resources=nodes/finalizers,verbs=update
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started Node Reconciler")
	n := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, n); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	nodeName := n.GetName()

	m := &machinev1.Machine{}
	if err := r.Client.Get(ctx, apitypes.NamespacedName{Name: nodeName, Namespace: machineNamespace}, m); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if m.Spec.ProviderID != nil {
		// We dont controll this resource, so we walk away
		return ctrl.Result{}, nil
	}

	if m.GetName() == nodeName {
		logger.Info("Found Matching Machine", "Machine", m)
		nref, err := reference.GetReference(scheme.Scheme, n)
		if err != nil {
			return ctrl.Result{}, err
		}
		m.Status.NodeRef = nref
		logger.Info("Updated Machine Status", "Status", m.Status)
		r.Client.Status().Update(ctx, m)
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
