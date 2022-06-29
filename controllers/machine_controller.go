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
	"strings"
	"time"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	requeueAfter     = 30 * time.Second
	machineNamespace = "openshift-machine-api"
)

// MachineReconciler reconciles a Machine object
type MachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=machine.openshift.io,resources=machines,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=machine.openshift.io,resources=machines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=machine.openshift.io,resources=machines/finalizers,verbs=update
func (r *MachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started Machine Reconciler")
	// Fetch the Machine instance
	m := &machinev1.Machine{}
	if err := r.Client.Get(ctx, req.NamespacedName, m); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if len(m.Status.Addresses) == 0 && m.Spec.ProviderID == nil {
		logger.Info("Adding Addresses to Machine Status", "Status", "m.Status")
		if err := r.AddStatusAddresses(m); err != nil {
			return ctrl.Result{}, nil
		}
		logger.Info("New Status", "Status", m.Status)
		r.Client.Status().Update(ctx, m)
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&machinev1.Machine{}).
		Complete(r)
}

func (r *MachineReconciler) AddStatusAddresses(m *machinev1.Machine) error {
	var a []corev1.NodeAddress
	machineName := m.GetName()
	// Set Hostname Address Type
	a = append(a,
		corev1.NodeAddress{
			Type:    corev1.NodeHostName,
			Address: machineName,
		},
	)
	// Set Internal DNS Address Type
	a = append(a,
		corev1.NodeAddress{
			Type:    corev1.NodeInternalDNS,
			Address: machineName,
		},
	)
	a = append(a,
		corev1.NodeAddress{
			Type:    corev1.NodeInternalDNS,
			Address: machineName + ".ec2.internal",
		},
	)
	// Set Internal IP Address Type
	a = append(a,
		corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: strings.Join(strings.Split(machineName, "-")[1:], "."),
		},
	)
	m.Status.Addresses = a
	return nil
}
