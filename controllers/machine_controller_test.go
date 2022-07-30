package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	machinev1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("Machine controller", func() {

	const (
		MachineName       = "test-machine"
		MachineHostname   = "testhost"
		MachineIP         = "1.2.3.4"
		MachineIPHostname = "ip-1-2-3-4"
		MachineNamespace  = "openshift-machine-api"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)
	Context("Updating Machine Status Addresses", func() {
		When("Machine Contains Proper Annotations", func() {
			var (
				rawMachine       *machinev1.Machine
				ctx              context.Context
				machineLookupKey = types.NamespacedName{Name: MachineName, Namespace: MachineNamespace}
			)
			BeforeEach(func() {
				By("By creating a new machine")
				ctx = context.Background()
				rawMachine = &machinev1.Machine{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "machine.openshift.io/v1beta1",
						Kind:       "Machine",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      MachineName,
						Namespace: MachineNamespace,
						Annotations: map[string]string{
							getAnnotationKey(InternalIPAnnotation):  MachineIP,
							getAnnotationKey(HostnameAnnotation):    MachineHostname,
							getAnnotationKey(InternalDNSAnnotation): fmt.Sprintf("%s.local", MachineHostname),
						},
					},
					Spec: machinev1.MachineSpec{},
					Status: machinev1.MachineStatus{
						Addresses: []corev1.NodeAddress{},
					},
				}
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, rawMachine)).Should(Succeed())
				Eventually(k8sClient.Get(ctx, machineLookupKey, &machinev1.Machine{})).ShouldNot(Succeed())
			})

			It("Should have the correct Status.Addresses", func() {
				Expect(k8sClient.Create(ctx, rawMachine)).Should(Succeed())

				createdMachine := &machinev1.Machine{}

				// We'll need to retry getting this newly created Machine, given that creation may not immediately happen.
				Eventually(func() bool {
					return k8sClient.Get(ctx, machineLookupKey, createdMachine) == nil
				}, timeout, interval).Should(BeTrue())

				Expect(createdMachine).ShouldNot(Equal(&machinev1.Machine{}))

				expectedAddresses := []corev1.NodeAddress{
					{
						Type:    corev1.NodeHostName,
						Address: MachineHostname,
					},
					{
						Type:    corev1.NodeInternalIP,
						Address: MachineIP,
					},
					{
						Type:    corev1.NodeInternalDNS,
						Address: MachineHostname,
					},
					{
						Type:    corev1.NodeInternalDNS,
						Address: fmt.Sprintf("%s.local", MachineHostname),
					},
				}

				Eventually(func() []corev1.NodeAddress {
					err := k8sClient.Get(ctx, machineLookupKey, createdMachine)
					if err != nil {
						return []corev1.NodeAddress{}
					}
					return createdMachine.Status.Addresses
				}, timeout, interval).Should(ContainElements(expectedAddresses))
			})

			It("Should preserve address fields of other types", func() {
				Expect(k8sClient.Create(ctx, rawMachine)).Should(Succeed())

				existingAddress := corev1.NodeAddress{
					Type:    corev1.NodeExternalIP,
					Address: "5.6.7.8",
				}

				createdMachine := &machinev1.Machine{}

				// We'll need to retry getting this newly created Machine, given that creation may not immediately happen.
				Eventually(func() bool {
					return k8sClient.Get(ctx, machineLookupKey, createdMachine) == nil
				}, timeout, interval).Should(BeTrue())

				Eventually(func() error {
					k8sClient.Get(ctx, machineLookupKey, createdMachine)
					createdMachine.Status.Addresses = append(createdMachine.Status.Addresses, *existingAddress.DeepCopy())
					return k8sClient.Status().Update(ctx, createdMachine)
				}, timeout, interval).Should(Succeed())

				Eventually(func() []corev1.NodeAddress {
					err := k8sClient.Get(ctx, machineLookupKey, createdMachine)
					if err != nil {
						return []corev1.NodeAddress{}
					}
					return createdMachine.Status.Addresses

				}, timeout, interval).Should(ContainElement(existingAddress))
			})

		})

		When("Machine Does Not Match", func() {
			var (
				rawMachine       *machinev1.Machine
				ctx              context.Context
				machineLookupKey = types.NamespacedName{Name: MachineName, Namespace: MachineNamespace}
			)
			BeforeEach(func() {
				By("By creating a new machine")
				ctx = context.Background()
				rawMachine = &machinev1.Machine{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "machine.openshift.io/v1beta1",
						Kind:       "Machine",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      MachineName,
						Namespace: MachineNamespace,
					},
					Spec: machinev1.MachineSpec{},
					Status: machinev1.MachineStatus{
						Addresses: []corev1.NodeAddress{},
					},
				}
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, rawMachine)).Should(Succeed())
				Eventually(k8sClient.Get(ctx, machineLookupKey, &machinev1.Machine{})).ShouldNot(Succeed())
			})

			It("Should not change the status", func() {
				Expect(k8sClient.Create(ctx, rawMachine)).Should(Succeed())

				createdMachine := &machinev1.Machine{}

				Eventually(func() bool {
					return k8sClient.Get(ctx, machineLookupKey, createdMachine) == nil
				}, timeout, interval).Should(BeTrue())

				Expect(createdMachine.Status.Addresses).Should(BeEmpty())

			})

		})

		When("Machine has a name matching regex", func() {
			var (
				rawMachine       *machinev1.Machine
				ctx              context.Context
				machineLookupKey = types.NamespacedName{Name: MachineIPHostname, Namespace: MachineNamespace}
			)
			BeforeEach(func() {
				By("By creating a new machine")
				ctx = context.Background()
				rawMachine = &machinev1.Machine{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "machine.openshift.io/v1beta1",
						Kind:       "Machine",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      MachineIPHostname,
						Namespace: MachineNamespace,
					},
					Spec: machinev1.MachineSpec{},
					Status: machinev1.MachineStatus{
						Addresses: []corev1.NodeAddress{},
					},
				}
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, rawMachine)).Should(Succeed())
				Eventually(k8sClient.Get(ctx, machineLookupKey, &machinev1.Machine{})).ShouldNot(Succeed())
			})

			It("Should have the correct status", func() {
				Expect(k8sClient.Create(ctx, rawMachine)).Should(Succeed())

				createdMachine := &machinev1.Machine{}

				Eventually(func() bool {
					return k8sClient.Get(ctx, machineLookupKey, createdMachine) == nil
				}, timeout, interval).Should(BeTrue())

				expectedAddresses := []corev1.NodeAddress{
					{
						Type:    corev1.NodeHostName,
						Address: MachineIPHostname,
					},
					{
						Type:    corev1.NodeInternalIP,
						Address: MachineIP,
					},
					{
						Type:    corev1.NodeInternalDNS,
						Address: MachineIPHostname,
					},
					{
						Type:    corev1.NodeInternalDNS,
						Address: fmt.Sprintf("%s.ec2.internal", MachineIPHostname),
					},
				}

				Eventually(func() []corev1.NodeAddress {
					err := k8sClient.Get(ctx, machineLookupKey, createdMachine)
					if err != nil {
						return []corev1.NodeAddress{}
					}
					return createdMachine.Status.Addresses
				}, timeout, interval).Should(ContainElements(expectedAddresses))
			})

		})

	})
	Context("Updating Machine Status ProviderStatus", func() {
		When("Machine Contains Proper Annotations", func() {
			var (
				rawMachine       *machinev1.Machine
				ctx              context.Context
				machineLookupKey = types.NamespacedName{Name: MachineName, Namespace: MachineNamespace}
				instanceState    = "teststate"
			)
			BeforeEach(func() {
				By("By creating a new machine")
				ctx = context.Background()
				rawMachine = &machinev1.Machine{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "machine.openshift.io/v1beta1",
						Kind:       "Machine",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      MachineName,
						Namespace: MachineNamespace,
						Annotations: map[string]string{
							getAnnotationKey(ProviderStateAnnotation): instanceState,
						},
					},
					Spec: machinev1.MachineSpec{},
					Status: machinev1.MachineStatus{
						Addresses: []corev1.NodeAddress{},
					},
				}
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, rawMachine)).Should(Succeed())
				Eventually(k8sClient.Get(ctx, machineLookupKey, &machinev1.Machine{})).ShouldNot(Succeed())
			})

			It("Should add the correct ProviderStatus", func() {
				Expect(k8sClient.Create(ctx, rawMachine)).Should(Succeed())

				createdMachine := &machinev1.Machine{}

				// We'll need to retry getting this newly created Machine, given that creation may not immediately happen.
				Eventually(func() bool {
					return k8sClient.Get(ctx, machineLookupKey, createdMachine) == nil
				}, timeout, interval).Should(BeTrue())

				Expect(createdMachine).ShouldNot(Equal(&machinev1.Machine{}))

				By("Creating ProviderStatus when machine annotation is created")
				expectedProviderStatus := &providerStatus{
					InstanceState: &instanceState,
					ProvidedBy:    &myProviderName,
				}
				Eventually(func() *providerStatus {
					err := k8sClient.Get(ctx, machineLookupKey, createdMachine)
					if err != nil {
						return &providerStatus{}
					}
					ps, err := providerStatusFromRawExtension(createdMachine.Status.ProviderStatus)
					if err != nil {
						return &providerStatus{}
					}
					return ps
				}, timeout, interval).Should(Equal(expectedProviderStatus))

				By("Updating ProviderStatus when machine annotation is different")
				var newstate = "newstate"
				diffProviderStatus := &providerStatus{
					InstanceState: &newstate,
					ProvidedBy:    &myProviderName,
				}
				var err error
				createdMachine.Status.ProviderStatus, err = diffProviderStatus.toRawExtension()
				Expect(err).Should(BeNil())
				Expect(k8sClient.Status().Update(ctx, createdMachine)).Should(Succeed())

				Eventually(func() *providerStatus {
					err := k8sClient.Get(ctx, machineLookupKey, createdMachine)
					if err != nil {
						return &providerStatus{}
					}
					ps, err := providerStatusFromRawExtension(createdMachine.Status.ProviderStatus)
					if err != nil {
						return &providerStatus{}
					}
					return ps
				}, timeout, interval).Should(Equal(expectedProviderStatus))

			})

			It("Should not modify ProviderStatus if it exists and was set by another process", func() {
				Expect(k8sClient.Create(ctx, rawMachine)).Should(Succeed())

				createdMachine := &machinev1.Machine{}

				// We'll need to retry getting this newly created Machine, given that creation may not immediately happen.
				Eventually(func() bool {
					return k8sClient.Get(ctx, machineLookupKey, createdMachine) == nil
				}, timeout, interval).Should(BeTrue())

				Expect(createdMachine).ShouldNot(Equal(&machinev1.Machine{}))

				expectedRawProviderStatus := []byte("{\"myKey\":\"myValue\"}")
				expectedProviderStatus := &runtime.RawExtension{
					Raw: expectedRawProviderStatus,
				}

				Eventually(func() error {
					k8sClient.Get(ctx, machineLookupKey, createdMachine)
					createdMachine.Status.ProviderStatus = expectedProviderStatus
					return k8sClient.Status().Update(ctx, createdMachine)
				}, timeout, interval).Should(Succeed())

				Consistently(func(g Gomega) {
					k8sClient.Get(ctx, machineLookupKey, createdMachine)
					g.Expect(createdMachine.Status.ProviderStatus).Should(Equal(expectedProviderStatus))
				}, timeout, interval).Should(Succeed())

			})
		})
	})
	Context("Manage Machine Status Phase", func() {
		When("Machine Contains Proper Annotations", func() {
			var (
				rawMachine       *machinev1.Machine
				rawNode          *corev1.Node
				ctx              context.Context
				machineLookupKey = types.NamespacedName{Name: MachineName, Namespace: MachineNamespace}
				instanceState    = "teststate"
			)
			BeforeEach(func() {
				By("By creating a new machine")
				ctx = context.Background()
				rawMachine = &machinev1.Machine{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "machine.openshift.io/v1beta1",
						Kind:       "Machine",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      MachineName,
						Namespace: MachineNamespace,
						Annotations: map[string]string{
							getAnnotationKey(PhaseAnnotation):      instanceState,
							getAnnotationKey(InternalIPAnnotation): MachineIP,
							getAnnotationKey(HostnameAnnotation):   MachineHostname,
						},
					},
					Spec: machinev1.MachineSpec{},
					Status: machinev1.MachineStatus{
						Addresses: []corev1.NodeAddress{},
					},
				}
				rawNode = &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
					Spec: corev1.NodeSpec{},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: MachineIP,
							},
						},
					},
				}

			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, rawMachine)).Should(Succeed())
				Eventually(k8sClient.Get(ctx, machineLookupKey, &machinev1.Machine{})).ShouldNot(Succeed())
			})

			It("Should set the phase correctly", func() {
				Expect(k8sClient.Create(ctx, rawMachine)).Should(Succeed())

				createdMachine := &machinev1.Machine{}

				// We'll need to retry getting this newly created Machine, given that creation may not immediately happen.
				Eventually(func() bool {
					return k8sClient.Get(ctx, machineLookupKey, createdMachine) == nil
				}, timeout, interval).Should(BeTrue())

				Expect(createdMachine).ShouldNot(Equal(&machinev1.Machine{}))
				By("Setting it to Provisioned when there is no nodeRef")
				Eventually(func() *string {
					k8sClient.Get(ctx, machineLookupKey, createdMachine)
					return createdMachine.Status.Phase
				}, timeout, interval).Should(HaveValue(Equal(phaseProvisioned)))

				By("Changing it to Running When the nodeRef is created")
				Expect(k8sClient.Create(ctx, rawNode)).Should(Succeed())
				// Eventually(func() error {
				// 	k8sClient.Get(ctx, machineLookupKey, createdMachine)
				// 	createdMachine.Status.NodeRef = &corev1.ObjectReference{
				// 		Kind: "Node",
				// 		Name: "test-node",
				// 	}
				// 	return k8sClient.Status().Update(ctx, createdMachine)
				// }).Should(Succeed())

				Eventually(func() *string {
					k8sClient.Get(ctx, machineLookupKey, createdMachine)
					return createdMachine.Status.Phase
				}, timeout, interval).Should(HaveValue(Equal(phaseRunning)))
				By("Changing it to Failed When the nodeRef points to a deleted node")
				Expect(k8sClient.Delete(ctx, rawNode)).Should(Succeed())
				Eventually(func() *string {
					k8sClient.Get(ctx, machineLookupKey, createdMachine)
					return createdMachine.Status.Phase
				}, timeout*3, interval).Should(HaveValue(Equal(phaseFailed)))
			})

		})
	})

})
