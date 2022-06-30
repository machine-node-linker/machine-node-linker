# machine-csr-noop

A simple controller to manage machines and link them to nodes

## Warnings

This controller makes **_A LOT_** of assumptions including, but possibly not limited to, the following:

- The Kubernetes API on the cluster is compatable with 1.24.2
- The Openshift API on the cluster is compatable with 4.10.15
- There is no machine-privider that would set conflictint settings
- An outside process is maching the `machine` resources will be created by something else
- The `node` hostname will match the name of the `machine` object
- That hostname will be consistent with the AWS EC2 IP based naming scheme.

Additionally, this controller does not currently support the following:

- Upgrades
- Configuration
- Alternative Namespaces

Finally, This is an ALPHA project at this time and was developed in 24 hours to fix an immediate need. This project may be abandoned or changed in ways that materially affect its operation.

## Usage

### Namespace

The Controller is intended to run in the `machine-csr-noop` namespace. It has not been tested in any other namespace

### Node Tolerations

The use of this controller in modifying `machine` objects for the purpose of having the `cluster-machine-approver-controller` approve CSRs requires that all parts of this
operator run on controlplane nodes. For some aspects of this, including the catalog and the controller, it is possible to be explicit with the tolerations. However, it is currently not possible to specify tolerations for the job pod that extracts the bundle and creates the `installplan` object. Because of this the toleration must be specified as an annotation on the namespace. [See the example ns.yaml](examples/ns.yaml)

### Examples

The files in the [examples directory](examples/) will result in a complete instalations based on the above assumptions.

## Legal

This product is not endorsed by, or supported by, Red Hat, Inc.

OpenShift is a registered trademark of Red Hat, Inc.
