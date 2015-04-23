# strive

A distributed scheduler for dynamic workfloads.

Based on the Google Omega scheduler design.

Requires:
* [vega](https://github.com/vektra/vega)

Features:
* Host based agent to track tasks
* Anti-entropy to keep cluster in proper state
* Support for running tasks in Docker as well as directly on hosts
* Cpu, Memory, Ports, Volumes, as well as arbitrary resources tracking


