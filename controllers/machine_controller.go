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
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	kjson "sigs.k8s.io/json"
)

const (
	requeueAfter            = 30 * time.Second
	machineNamespace        = "openshift-machine-api"
	AnnotationBase          = "machine-node-linker.github.com"
	InternalIPAnnotation    = "internal-ip"
	InternalDNSAnnotation   = "internal-dns"
	HostnameAnnotation      = "hostname"
	ProviderStateAnnotation = "provider-state"
	PhaseAnnotation         = "manage-phase"

	// This operator supports a subset of phase settings.
	// When a noderef exists and points to a non-existant node
	// When we have a previous state of Running and no noderef
	phaseFailed = "Failed"

	// Used between when a machine is created, and a noderef exists
	// Machine has been given address
	// Machine has NOT been given nodeRef
	phaseProvisioned = "Provisioned"

	// Instance exists
	// Machine has been given providerID/address
	// Machine has been given a nodeRef
	phaseRunning = "Running"
)

var (
	//Provides hostname match for migrations from machine-csr-noop operator
	//Matches hostnames in the format of AWS ip based hostname assignment
	// Ex. ip-192-168-1-150
	LegacyHostnameRegex = regexp.MustCompile("ip(-(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}")
	myProviderName      = AnnotationBase
)

// Object for serializing providerstatus object in machine status
// API defines it as a RawExtension
type providerStatus struct {
	InstanceState *string `json:"instanceState,omitempty"`
	ProvidedBy    *string `json:"providedBy,omitempty"`
}

// MachineReconciler reconciles a Machine object
type MachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=machine.openshift.io,resources=machines,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=machine.openshift.io,resources=machines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=machine.openshift.io,resources=machines/finalizers,verbs=update
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
		return ctrl.Result{}, fmt.Errorf("unable to get machine: %v", err)
	}
	modAddr, err := r.AddStatusAddressesFromAnnotations(m.Annotations)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to parse adress annotations: %v", err)
	}

	if len(modAddr) == 0 && LegacyHostnameRegex.Match([]byte(m.GetName())) && len(m.Status.Addresses) == 0 && m.Spec.ProviderID == nil {
		modAddr, err = r.AddStatusAddressesFromHostname(m.GetName())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to process addresses from hostname: %v", err)
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
	// If phase management key is set,  we will manage the phase
	if _, ok := m.Annotations[getAnnotationKey(PhaseAnnotation)]; ok {
		phase, err := r.setPhase(m, ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to parse phase status: %v", err)
		}
		if !reflect.DeepEqual(m.Status.Phase, &phase) {
			m.Status.Phase = &phase
			logger.Info("New Phase", "Status", m.Status)
			r.Client.Status().Update(ctx, m)
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
		}
	}

	if value, ok := m.Annotations[getAnnotationKey(ProviderStateAnnotation)]; ok {
		var ps *providerStatus
		var err error
		if ps, err = providerStatusFromRawExtension(m.Status.ProviderStatus); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to parse provider status: %v", err)
		}
		if ps.ProvidedBy != nil && *ps.ProvidedBy != AnnotationBase {
			return ctrl.Result{Requeue: false}, fmt.Errorf("refusing to change provider status: %v", ps)
		}
		newPs := &providerStatus{
			InstanceState: &value,
			ProvidedBy:    &myProviderName,
		}
		if !(reflect.DeepEqual(ps, newPs)) {

			if m.Status.ProviderStatus, err = newPs.toRawExtension(); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to create RawExtension: %v", err)
			}
			logger.Info("New providerStatus", "Status", m.Status)
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

// Create a slice of NodeAddress objects based on a hostname matching ip-x-x-x-x
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

// Create a slice of NodeAddress objects based on annotations using machine-node-linker.github.com/ prefix
func (r *MachineReconciler) AddStatusAddressesFromAnnotations(annotations map[string]string) ([]corev1.NodeAddress, error) {
	var addr []corev1.NodeAddress

	if value, ok := annotations[getAnnotationKey(InternalIPAnnotation)]; ok {
		addr = append(addr, corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: value,
		})
	}
	if value, ok := annotations[getAnnotationKey(HostnameAnnotation)]; ok {
		addr = append(addr, corev1.NodeAddress{
			Type:    corev1.NodeHostName,
			Address: value,
		}, corev1.NodeAddress{
			Type:    corev1.NodeInternalDNS,
			Address: value,
		})
	}
	if value, ok := annotations[getAnnotationKey(InternalDNSAnnotation)]; ok {
		addr = append(addr, corev1.NodeAddress{
			Type:    corev1.NodeInternalDNS,
			Address: value,
		})
	}
	return addr, nil
}

// determine the new phase based on node status and current phase
// this function should only be called if we are responsible for setting phase
func (r *MachineReconciler) setPhase(m *machinev1.Machine, ctx context.Context) (string, error) {
	var currentPhase *string = m.Status.Phase
	if currentPhase == nil {
		return phaseProvisioned, nil
	}
	if *currentPhase == phaseFailed {
		return phaseFailed, nil
	}
	if m.Status.NodeRef != nil && !reflect.DeepEqual(m.Status.NodeRef, &corev1.ObjectReference{}) {
		ns := apitypes.NamespacedName{
			Name: m.Status.NodeRef.Name,
		}
		n := &corev1.Node{}
		if err := r.Client.Get(ctx, ns, n); err != nil {
			if apierrors.IsNotFound(err) {
				return phaseFailed, nil
			}
			return "", fmt.Errorf("unable to get node: %v", err)
		}
		if *currentPhase == phaseProvisioned || *currentPhase == phaseRunning {
			return phaseRunning, nil
		}
	}
	if *currentPhase != phaseProvisioned {
		return phaseFailed, nil
	}
	return *currentPhase, nil
}

func getAnnotationKey(key string) string {
	return fmt.Sprintf("%s/%s", AnnotationBase, key)
}

func providerStatusFromRawExtension(raw *runtime.RawExtension) (*providerStatus, error) {
	if raw == nil {
		return &providerStatus{}, nil
	}

	ps := &providerStatus{}
	strict, err := kjson.UnmarshalStrict(raw.Raw, ps)
	if err != nil {
		return nil, fmt.Errorf("unable to create providerStatus from RawExtension %v", err)
	}
	if len(strict) > 0 {
		return nil, fmt.Errorf("unable to create providerStatus from RawExtension %v", strict)
	}
	return ps, nil
}

func (ps *providerStatus) toRawExtension() (*runtime.RawExtension, error) {
	if ps == nil {
		return &runtime.RawExtension{}, nil
	}

	var rawBytes []byte
	var err error
	if rawBytes, err = json.Marshal(ps); err != nil {
		return nil, fmt.Errorf("error marshalling providerStatus: %v", err)
	}

	return &runtime.RawExtension{
		Raw: rawBytes,
	}, nil
}
