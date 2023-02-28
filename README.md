# astertower
The operator to control workflow and astermule, a lightweight microservice composition workflow implement.

The core function of the Astertower control plane is to register the custom resource Astro(a Kubernetes custom resource that describes workflows and complies with the Kubernetes API extension standard) and the custom controller inside Kubernetes. Astertower will manage workflow choreography in a Kubernetes native way.
![architecture](docs/img/astertower.png)

The Astertower control plane must ensure that all required microservices are in place before starting the workflow engine Astermule. So the Astertower controller creates the Deployment instance and Service corresponding to the workflow node in Kubernetes each time it creates an Astro workflow resource. And add the Owner Reference to it so that Kubernetes can implement garbage collection after the workflow is deleted.

## Getting Started
The astertower project relies on the Kubernetes cluster. For details, see [astertower-tutorial](https://github.com/kasterism/astertower/blob/main/docs/img/astertower-tutorial.md).
