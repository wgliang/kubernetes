/*
Copyright 2015 The Kubernetes Authors.

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

package node

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// DefaultBindAllHostIP defines the default ip address used to bind to all host.
const DefaultBindAllHostIP = "0.0.0.0"

// CreateNodeNameToInfoMap obtains a list of pods and pivots that list into a map where the keys are node names
// and the values are the aggregated information for that node.
func CreateNodeNameToInfoMap(pods []*v1.Pod, nodes []*v1.Node) map[string]*NodeInfo {
	nodeNameToInfo := make(map[string]*NodeInfo)
	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		if _, ok := nodeNameToInfo[nodeName]; !ok {
			nodeNameToInfo[nodeName] = NewNodeInfo()
		}
		nodeNameToInfo[nodeName].AddPod(pod)
	}
	imageExistenceMap := createImageExistenceMap(nodes)

	for _, node := range nodes {
		if _, ok := nodeNameToInfo[node.Name]; !ok {
			nodeNameToInfo[node.Name] = NewNodeInfo()
		}
		nodeInfo := nodeNameToInfo[node.Name]
		nodeInfo.SetNode(node)
		nodeInfo.imageStates = getNodeImageStates(node, imageExistenceMap)
	}
	return nodeNameToInfo
}

// getNodeImageStates returns the given node's image states based on the given imageExistence map.
func getNodeImageStates(node *v1.Node, imageExistenceMap map[string]sets.String) map[string]*ImageStateSummary {
	imageStates := make(map[string]*ImageStateSummary)

	for _, image := range node.Status.Images {
		for _, name := range image.Names {
			imageStates[name] = &ImageStateSummary{
				Size:     image.SizeBytes,
				NumNodes: len(imageExistenceMap[name]),
			}
		}
	}
	return imageStates
}

// createImageExistenceMap returns a map recording on which nodes the images exist, keyed by the images' names.
func createImageExistenceMap(nodes []*v1.Node) map[string]sets.String {
	imageExistenceMap := make(map[string]sets.String)
	for _, node := range nodes {
		for _, image := range node.Status.Images {
			for _, name := range image.Names {
				if _, ok := imageExistenceMap[name]; !ok {
					imageExistenceMap[name] = sets.NewString(node.Name)
				} else {
					imageExistenceMap[name].Insert(node.Name)
				}
			}
		}
	}
	return imageExistenceMap
}

// ProtocolPort represents a protocol port pair, e.g. tcp:80.
type ProtocolPort struct {
	Protocol string
	Port     int32
}

// NewProtocolPort creates a ProtocolPort instance.
func NewProtocolPort(protocol string, port int32) *ProtocolPort {
	pp := &ProtocolPort{
		Protocol: protocol,
		Port:     port,
	}

	if len(pp.Protocol) == 0 {
		pp.Protocol = string(v1.ProtocolTCP)
	}

	return pp
}

// HostPortInfo stores mapping from ip to a set of ProtocolPort
type HostPortInfo map[string]map[ProtocolPort]struct{}

// Add adds (ip, protocol, port) to HostPortInfo
func (h HostPortInfo) Add(ip, protocol string, port int32) {
	if port <= 0 {
		return
	}

	h.sanitize(&ip, &protocol)

	pp := NewProtocolPort(protocol, port)
	if _, ok := h[ip]; !ok {
		h[ip] = map[ProtocolPort]struct{}{
			*pp: {},
		}
		return
	}

	h[ip][*pp] = struct{}{}
}

// Remove removes (ip, protocol, port) from HostPortInfo
func (h HostPortInfo) Remove(ip, protocol string, port int32) {
	if port <= 0 {
		return
	}

	h.sanitize(&ip, &protocol)

	pp := NewProtocolPort(protocol, port)
	if m, ok := h[ip]; ok {
		delete(m, *pp)
		if len(h[ip]) == 0 {
			delete(h, ip)
		}
	}
}

// Len returns the total number of (ip, protocol, port) tuple in HostPortInfo
func (h HostPortInfo) Len() int {
	length := 0
	for _, m := range h {
		length += len(m)
	}
	return length
}

// CheckConflict checks if the input (ip, protocol, port) conflicts with the existing
// ones in HostPortInfo.
func (h HostPortInfo) CheckConflict(ip, protocol string, port int32) bool {
	if port <= 0 {
		return false
	}

	h.sanitize(&ip, &protocol)

	pp := NewProtocolPort(protocol, port)

	// If ip is 0.0.0.0 check all IP's (protocol, port) pair
	if ip == DefaultBindAllHostIP {
		for _, m := range h {
			if _, ok := m[*pp]; ok {
				return true
			}
		}
		return false
	}

	// If ip isn't 0.0.0.0, only check IP and 0.0.0.0's (protocol, port) pair
	for _, key := range []string{DefaultBindAllHostIP, ip} {
		if m, ok := h[key]; ok {
			if _, ok2 := m[*pp]; ok2 {
				return true
			}
		}
	}

	return false
}

// sanitize the parameters
func (h HostPortInfo) sanitize(ip, protocol *string) {
	if len(*ip) == 0 {
		*ip = DefaultBindAllHostIP
	}
	if len(*protocol) == 0 {
		*protocol = string(v1.ProtocolTCP)
	}
}
