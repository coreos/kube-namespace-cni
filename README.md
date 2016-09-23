# Kubernetes Per-Namespace CNI Configuration

This is a CNI plugin for use with Kubernetes.  It takes a mapping of
namespaces to CNI network configurations, with an optional default,
and delegates to other CNI plugins to set up networking.  You might
use this to place pods in a particular namespace onto an isolated
network.
