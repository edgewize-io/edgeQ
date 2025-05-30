
# Edgewize EdgeQ

`Edgewize EdgeQ` is an inference service governance framework that supports multiple model services. It enhances the management of inference services by providing capabilities such as multi-model sharing, traffic governance, auditing, security, and scalability.

[English](README.md) | [中文](README_cn.md)

## Overview

`Edgewize EdgeQ` addresses the following key challenges:

* **Multi-Model Hardware Sharing**: Efficiently manage and share hardware resources among multiple models.
* **Hierarchical Computing Power Supply**: Provide a hierarchical allocation of computational resources for models.
* **Model Privacy**: Ensure the privacy and security of models.
* **Business Integration**: Facilitate seamless integration with various business workflows.
* **Cloud-Edge Computing Power Collaboration**: Enable collaborative computing power utilization between the cloud and edge.

## Key Features

* **Multi-Model Sharing**: Support the efficient sharing of resources among different models, optimizing hardware utilization.
* **Traffic Governance**: Implement effective traffic governance to manage the flow of requests and responses between models and clients.
* **Auditing**: Enable auditing capabilities to track and monitor model inference activities for compliance and analysis.
* **Security**: Implement security measures to safeguard models and data throughout the inference process.
* **Scalability**: Provide scalability features to accommodate the growing demands of model inference workloads.

## Getting Started

Enable ip forward on Edge Node
```
$ sudo echo "net.ipv4.ip_forward = 1" >> /etc/sysctl.conf
$ sudo sysctl -p
```

Then check it:
```
$ sudo sysctl -p | grep ip_forward
net.ipv4.ip_forward = 1
```

## Community and Support

For discussions, support, and collaboration, you can engage with the community through the following channels:

* [GitHub Repository](https://github.com/edgewize/edgeQ)
* [Community Forum](https://community.edgewize.com/)

## Contributions

Contributions to `Edgewize ModelMesh` are welcome. If you encounter issues or have suggestions, feel free to create an issue or submit a pull request on the GitHub repository.

## License

`Edgewize EdgeQ` is licensed under the [MIT License](LICENSE).



