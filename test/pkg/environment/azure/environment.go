/*
Portions Copyright (c) Microsoft Corporation.

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

package azure

import (
	"testing"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/scheme"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/Azure/karpenter-provider-azure/pkg/apis"
	"github.com/Azure/karpenter-provider-azure/pkg/apis/v1alpha2"
	"github.com/Azure/karpenter-provider-azure/pkg/consts"
	"github.com/Azure/karpenter-provider-azure/pkg/test"
	"github.com/Azure/karpenter-provider-azure/test/pkg/environment/common"
)

func init() {
	lo.Must0(apis.AddToScheme(scheme.Scheme))
	corev1beta1.NormalizedLabels = lo.Assign(corev1beta1.NormalizedLabels, map[string]string{"topology.disk.csi.azure.com/zone": v1.LabelTopologyZone})
}

const (
	WindowsDefaultImage      = "mcr.microsoft.com/oss/kubernetes/pause:3.9"
	CiliumAgentNotReadyTaint = "node.cilium.io/agent-not-ready"
)

type Environment struct {
	*common.Environment
	Region string
}

func NewEnvironment(t *testing.T) *Environment {
	env := common.NewEnvironment(t)

	return &Environment{
		Region:      "westus2",
		Environment: env,
	}
}

func (env *Environment) DefaultNodePool(nodeClass *v1alpha2.AKSNodeClass) *corev1beta1.NodePool {
	nodePool := coretest.NodePool()
	nodePool.Spec.Template.Spec.NodeClassRef = &corev1beta1.NodeClassReference{
		Name: nodeClass.Name,
	}
	nodePool.Spec.Template.Spec.Requirements = []corev1beta1.NodeSelectorRequirementWithMinValues{
		{NodeSelectorRequirement: v1.NodeSelectorRequirement{
			Key:      v1.LabelOSStable,
			Operator: v1.NodeSelectorOpIn,
			Values:   []string{string(v1.Linux)},
		}},
		{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      corev1beta1.CapacityTypeLabelKey,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{corev1beta1.CapacityTypeOnDemand},
			}},
		{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{corev1beta1.ArchitectureAmd64},
			}},
		{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      v1alpha2.LabelSKUFamily,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"D"},
			}},
	}
	nodePool.Spec.Disruption.ConsolidateAfter = &corev1beta1.NillableDuration{}
	nodePool.Spec.Disruption.ExpireAfter.Duration = nil
	nodePool.Spec.Limits = corev1beta1.Limits(v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("100"),
		v1.ResourceMemory: resource.MustParse("1000Gi"),
	})

	// TODO: make this conditional on Cilium
	// https://karpenter.sh/docs/concepts/nodepools/#cilium-startup-taint
	nodePool.Spec.Template.Spec.StartupTaints = append(nodePool.Spec.Template.Spec.StartupTaints, v1.Taint{
		Key:    CiliumAgentNotReadyTaint,
		Effect: v1.TaintEffectNoExecute,
		Value:  "true",
	})
	// # required for Karpenter to predict overhead from cilium DaemonSet
	nodePool.Spec.Template.ObjectMeta.Labels = lo.Assign(nodePool.Spec.Template.ObjectMeta.Labels, map[string]string{
		"kubernetes.azure.com/ebpf-dataplane": consts.NetworkDataplaneCilium,
	})
	return nodePool
}

func (env *Environment) ArmNodepool(nodeClass *v1alpha2.AKSNodeClass) *corev1beta1.NodePool {
	nodePool := env.DefaultNodePool(nodeClass)
	coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithMinValues{
		NodeSelectorRequirement: v1.NodeSelectorRequirement{
			Key:      v1.LabelArchStable,
			Operator: v1.NodeSelectorOpIn,
			Values:   []string{corev1beta1.ArchitectureArm64},
		}})
	return nodePool
}

func (env *Environment) DefaultAKSNodeClass() *v1alpha2.AKSNodeClass {
	nodeClass := test.AKSNodeClass()
	return nodeClass
}

func (env *Environment) AZLinuxNodeClass() *v1alpha2.AKSNodeClass {
	nodeClass := env.DefaultAKSNodeClass()
	nodeClass.Spec.ImageFamily = lo.ToPtr(v1alpha2.AzureLinuxImageFamily)
	return nodeClass
}
