# MacOS Actions Runner Controller
A controller for autoscaling self-hosted GitHub Actions runners on macOS systems.

## Overview
The macOS GitHub Actions Runner Controller is a solution for dynamically managing GitHub Actions runners on macOS environments. While actions-runner-controller focuses on Linux runners deployed in Kubernetes, this project specializes in macOS runners, addressing the unique challenges of macOS virtualization and Apple's licensing terms.
This controller:

- Dynamically scales macOS runners based on demand
- Manages the lifecycle of macOS GitHub Actions runners
- Ensures efficient resource utilization
- Distribute runners across multiple macOS nodes

## How it Works

1. Runner Requests: Runner requests are received through AWS SQS and processed by the Controller
2. Node Management: macOS nodes register with the Controller and periodically send heartbeats
3. Scheduling: The Controller schedules pending runners to appropriate nodes based on capacity
4. Runner Execution: Tarter agents on macOS nodes spawn and manage runner VMs using Tart
5. Lifecycle Management: The system manages the full lifecycle of runners, from creation to termination

## Key Features

- Autoscaling: Automatically adjust capacity based on runners demand
- Resource Efficiency: Optimize resource utilization by distributing runners across nodes
- Queue Management: Reduce macOS runners queue time
- Resiliency: Handle node failures and runner errors gracefully
- Monitoring: Comprehensive status and health monitoring of nodes and runners

## Acknowledgments

This project was inspired by and builds upon concepts from [actions-runner-controller](https://github.com/actions/actions-runner-controller), adapting them for the specific needs of macOS environments.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For more details on contributing to the project, please refer to "[Contributing Guide](./CONTRIBUTING.md)"

## Troubleshooting

We are very happy to help you with any issues you have. Feel free to reach out to us on GitHub or open an issue.