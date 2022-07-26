# machine-node-linker

A simple controller to manage machines and link them to nodes

## Warnings

This controller makes **_A LOT_** of assumptions including, but possibly not limited to, the following:

- The Kubernetes API on the cluster is compatable with 1.24.2
- The Openshift API on the cluster is compatable with 4.10.15
- There is no machine-privider that would set conflictint settings
- An outside process is macking the `machine` resources will be created by something else

Additionally, this controller does not currently support the following:

- Configuration

Finally, This is an ALPHA project at this time and was developed in 24 hours to fix an immediate need. This project may be abandoned or changed in ways that materially affect its operation. While there has been a major update which makes it both safer and more functional, that does not change the above warning

## Usage

### Annotations

The primary method of action for this controller is annotations on machine.machine.openshift.io/v1 objects.
The values of the following annotations will be copied into NodeAddress objects in the status.addresses object of the machine

| Annotation Key                              | NodeAddressType           | Value Expected               |
| ------------------------------------------- | ------------------------- | ---------------------------- |
| machine-node-linker.github.com/internal-ip  | InternalIP                | ip address (ex. 10.0.0.1 )   |
| machine-node-linker.github.com/internal-dns | InternalDNS               | fqdn (ex. node.my.domain )   |
| machine-node-linker.github.com/hostname     | Hostname<br \>InternalDNS | hostname (ex. nodehostname ) |

### Namespace

The Controller is intended to run in the `machine-node-linker` namespace. However, It should run in any namespace without issue. Users may be inclined to run this in a namespace with the openshift- or kube- prefixes in order to have the logs treated as infra logs rather than app logs. This is officially discouraged and cluster updates could cause this to break. Officially those prefixes are reserved by Openshift and should not be used for anything without explicit instruction in the openshift documentation or a RedHat supported operator.

### Node Tolerations

The use of this controller in modifying `machine` objects for the purpose of having the `cluster-machine-approver-controller` approve CSRs requires that all parts of this
operator run on controlplane nodes. For some aspects of this, including the catalog and the controller, it is possible to be explicit with the tolerations. However, it is currently not possible to specify tolerations for the job pod that extracts the bundle and creates the `installplan` object. Because of this the toleration must be specified as an annotation on the namespace. [See the example ns.yaml](examples/ns.yaml)

### Other

Labels and Taints from the Machine Spec will get copied by the machine-api-operator nodelink controller and so can be used. Additionally there exist a series of annotations on machine objects that provide special meaning to openshift. There is nothing in this operator that precludes those from working as well.

### LEGACY Config

If the following conditions are met, the operator will function.

- The annotations described in the top of this section are not used.
- The machine name is consistent with the AWS EC2 IP based naming scheme.
- The Machine does not have a provider ID
- The node does not have a provider ID when it joins

In this state, the following Address Objects will be made
| Type | Address |
| --- | --- |
| InternalIP | <ip from machine name> |
| Hostname | <machine name> |
| InternalDNS | <machine name> |
| InternalDNS | <machine name>.ec2.internal |

### Examples

The files in the [examples directory](examples/) will result in a complete instalations based on the above assumptions.

## Legal

This product is not endorsed by, or supported by, Red Hat, Inc.

OpenShift is a registered trademark of Red Hat, Inc.
