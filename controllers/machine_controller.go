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
	"fmt"
	"reflect"
	"regexp"
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
	requeueAfter          = 30 * time.Second
	machineNamespace      = "openshift-machine-api"
	AnnotationBase        = "machine-node-linker.github.com"
	InternalIPAnnotation  = "internal-ip"
	InternalDNSAnnotation = "internal-dns"
	HostnameAnnotation    = "hostname"
)

var LegacyHostnameRegex = regexp.MustCompile("ip(-(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}")

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

	modAddr, err := r.AddStatusAddressesFromAnnotations(m.Annotations)
	if err != nil {
		return ctrl.Result{}, err
	}

	if len(modAddr) == 0 && LegacyHostnameRegex.Match([]byte(m.GetName())) && len(m.Status.Addresses) == 0 && m.Spec.ProviderID == nil {
		modAddr, err = r.AddStatusAddressesFromHostname(m.GetName())
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if len(modAddr) > 0 {
		// Add addresses from machine.Status.Addresses that we dont create if they exist to prevent trashing
		for _, eAddr := range m.Status.Addresses {
			typeFound := false
			for _, mAddr := range modAddr {
				if eAddr.Type == mAddr.Type {
					typeFound = true
					break
				}
			}
			if !typeFound {
				modAddr = append(modAddr, *eAddr.DeepCopy())
			}
		}

		if !(reflect.DeepEqual(modAddr, m.Status.Addresses)) {
			logger.Info("Adding Addresses to Machine Status", "Status", "m.Status")
			m.Status.Addresses = modAddr
			logger.Info("New Status", "Status", m.Status)
			r.Client.Status().Update(ctx, m)
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&machinev1.Machine{}).
		Complete(r)
}

func (r *MachineReconciler) AddStatusAddressesFromHostname(machineName string) ([]corev1.NodeAddress, error) {
	var a []corev1.NodeAddress
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
	return a, nil
}

func (r *MachineReconciler) AddStatusAddressesFromAnnotations(annotations map[string]string) ([]corev1.NodeAddress, error) {
	var addr []corev1.NodeAddress

	if value, ok := annotations[fmt.Sprintf("%s/%s", AnnotationBase, InternalIPAnnotation)]; ok {
		addr = append(addr, corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: value,
		})
	}
	if value, ok := annotations[fmt.Sprintf("%s/%s", AnnotationBase, HostnameAnnotation)]; ok {
		addr = append(addr, corev1.NodeAddress{
			Type:    corev1.NodeHostName,
			Address: value,
		}, corev1.NodeAddress{
			Type:    corev1.NodeInternalDNS,
			Address: value,
		})
	}
	if value, ok := annotations[fmt.Sprintf("%s/%s", AnnotationBase, InternalDNSAnnotation)]; ok {
		addr = append(addr, corev1.NodeAddress{
			Type:    corev1.NodeInternalDNS,
			Address: value,
		})
	}
	return addr, nil
}
