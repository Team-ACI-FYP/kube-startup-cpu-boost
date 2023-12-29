// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package boost_test

import (
	"context"
	"time"

	autoscaling "github.com/google/kube-startup-cpu-boost/api/v1alpha1"
	cpuboost "github.com/google/kube-startup-cpu-boost/internal/boost"
	"github.com/google/kube-startup-cpu-boost/internal/boost/duration"
	"github.com/google/kube-startup-cpu-boost/internal/boost/resource"
	"github.com/google/kube-startup-cpu-boost/internal/mock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apiResource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("StartupCPUBoost", func() {
	var (
		spec  *autoscaling.StartupCPUBoost
		boost cpuboost.StartupCPUBoost
		err   error
		pod   *corev1.Pod
	)
	BeforeEach(func() {
		pod = podTemplate.DeepCopy()
		spec = specTemplate.DeepCopy()
	})
	Describe("Instantiates from the API specification", func() {
		JustBeforeEach(func() {
			boost, err = cpuboost.NewStartupCPUBoost(nil, spec)
		})
		It("does not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
		It("returns valid name", func() {
			Expect(boost.Name()).To(Equal(spec.Name))
		})
		It("returns valid namespace", func() {
			Expect(boost.Namespace()).To(Equal(spec.Namespace))
		})
		When("the spec has resource policy for containers", func() {
			var (
				containerOneName            = "container-one"
				containerTwoName            = "container-two"
				containerOnePercValue int64 = 120
				containerTwoFixedReq        = apiResource.MustParse("1")
				containerTwoFixedLim        = apiResource.MustParse("2")
			)
			BeforeEach(func() {
				spec.Spec.ResourcePolicy = autoscaling.ResourcePolicy{
					ContainerPolicies: []autoscaling.ContainerPolicy{
						{
							ContainerName: containerOneName,
							PercentageIncrease: &autoscaling.PercentageIncrease{
								Value: containerOnePercValue,
							},
						},
						{
							ContainerName: containerTwoName,
							FixedResources: &autoscaling.FixedResources{
								Requests: containerTwoFixedReq,
								Limits:   containerTwoFixedLim,
							},
						},
					},
				}
			})
			It("does not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
			It("returns valid resource policy for container one", func() {
				p, ok := boost.ResourcePolicy(containerOneName)
				Expect(ok).To(BeTrue())
				Expect(p).To(BeAssignableToTypeOf(&resource.PercentageContainerPolicy{}))
				percPolicy, _ := p.(*resource.PercentageContainerPolicy)
				Expect(percPolicy.Percentage()).To(Equal(containerOnePercValue))
			})
			It("returns valid resource policy for container two", func() {
				p, ok := boost.ResourcePolicy(containerTwoName)
				Expect(ok).To(BeTrue())
				Expect(p).To(BeAssignableToTypeOf(&resource.FixedPolicy{}))
				fixedPolicy, _ := p.(*resource.FixedPolicy)
				Expect(fixedPolicy.Requests()).To(Equal(containerTwoFixedReq))
				Expect(fixedPolicy.Limits()).To(Equal(containerTwoFixedLim))
			})
		})
		When("the spec has container policy without resource policy", func() {
			BeforeEach(func() {
				spec.Spec.ResourcePolicy = autoscaling.ResourcePolicy{
					ContainerPolicies: []autoscaling.ContainerPolicy{
						{
							ContainerName: "container-one",
						},
					},
				}
			})
			It("errors", func() {
				Expect(err).To(HaveOccurred())
			})
		})
		When("the spec has container policy with two resource policies", func() {
			BeforeEach(func() {
				spec.Spec.ResourcePolicy = autoscaling.ResourcePolicy{
					ContainerPolicies: []autoscaling.ContainerPolicy{
						{
							ContainerName: "container-one",
						},
					},
				}
			})
			It("errors", func() {
				Expect(err).To(HaveOccurred())
			})
		})
		When("the spec has fixed duration policy", func() {
			BeforeEach(func() {
				spec.Spec.DurationPolicy.Fixed = &autoscaling.FixedDurationPolicy{
					Unit:  autoscaling.FixedDurationPolicyUnitSec,
					Value: 123,
				}
			})
			It("returns fixed duration policy implementation", func() {
				Expect(boost.DurationPolicies()).To(HaveKey(duration.FixedDurationPolicyName))
			})
			It("returned fixed duration policy implementation is valid", func() {
				p := boost.DurationPolicies()[duration.FixedDurationPolicyName]
				fixedP, ok := p.(*duration.FixedDurationPolicy)
				Expect(ok).To(BeTrue())
				expDuration := time.Duration(spec.Spec.DurationPolicy.Fixed.Value) * time.Second
				Expect(fixedP.Duration()).To(Equal(expDuration))
			})
		})
		When("the spec has pod condition duration policy", func() {
			BeforeEach(func() {
				spec.Spec.DurationPolicy.Fixed = &autoscaling.FixedDurationPolicy{
					Unit:  autoscaling.FixedDurationPolicyUnitSec,
					Value: 123,
				}
				spec.Spec.DurationPolicy.PodCondition = &autoscaling.PodConditionDurationPolicy{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}
			})
			It("returns pod condition duration policy implementation", func() {
				Expect(boost.DurationPolicies()).To(HaveKey(duration.PodConditionPolicyName))
			})
			It("returned pod condition duration policy implementation is valid", func() {
				p := boost.DurationPolicies()[duration.PodConditionPolicyName]
				podCondP, ok := p.(*duration.PodConditionPolicy)
				Expect(ok).To(BeTrue())
				Expect(podCondP.Condition()).To(Equal(spec.Spec.DurationPolicy.PodCondition.Type))
				Expect(podCondP.Status()).To(Equal(spec.Spec.DurationPolicy.PodCondition.Status))
			})
		})
	})

	Describe("Upserts a POD", func() {
		var (
			mockCtrl   *gomock.Controller
			mockClient *mock.MockClient
		)
		BeforeEach(func() {
			mockCtrl = gomock.NewController(GinkgoT())
			mockClient = mock.NewMockClient(mockCtrl)
		})
		JustBeforeEach(func() {
			boost, err = cpuboost.NewStartupCPUBoost(mockClient, spec)
			Expect(err).ShouldNot(HaveOccurred())
		})
		When("POD does not exist", func() {
			JustBeforeEach(func() {
				err = boost.UpsertPod(context.TODO(), pod)
			})
			It("doesn't error", func() {
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("stores a POD", func() {
				p, ok := boost.Pod(pod.Name)
				Expect(ok).To(BeTrue())
				Expect(p.Name).To(Equal(pod.Name))
			})
		})
		When("POD exists", func() {
			var existingPod *corev1.Pod
			var createTimestamp metav1.Time
			BeforeEach(func() {
				existingPod = podTemplate.DeepCopy()
				createTimestamp = metav1.NewTime(time.Now())
				pod.CreationTimestamp = createTimestamp
			})
			JustBeforeEach(func() {
				err = boost.UpsertPod(context.TODO(), existingPod)
				Expect(err).ShouldNot(HaveOccurred())
				err = boost.UpsertPod(context.TODO(), pod)
			})
			It("doesn't error", func() {
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("stores an updated POD", func() {
				p, found := boost.Pod(pod.Name)
				Expect(found).To(BeTrue())
				Expect(p.Name).To(Equal(pod.Name))
				Expect(p.CreationTimestamp).To(Equal(createTimestamp))
			})
			When("boost spec has pod condition policy", func() {
				BeforeEach(func() {
					spec.Spec.DurationPolicy.PodCondition = &autoscaling.PodConditionDurationPolicy{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					}
				})
				When("POD condition matches spec policy", func() {
					BeforeEach(func() {
						pod.Status.Conditions = []corev1.PodCondition{{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						}}
						mockClient.EXPECT().
							Update(gomock.Any(), gomock.Eq(pod)).
							Return(nil)
					})
					It("doesn't error", func() {
						Expect(err).NotTo(HaveOccurred())
					})
				})
				When("POD condition does not match spec policy", func() {
					BeforeEach(func() {
						pod.Status.Conditions = []corev1.PodCondition{{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						}}
					})
					It("doesn't error", func() {
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})
		})
	})

	Describe("Deletes a pod", func() {
		JustBeforeEach(func() {
			boost, err = cpuboost.NewStartupCPUBoost(nil, spec)
			Expect(err).ShouldNot(HaveOccurred())
		})
		When("Pod exists", func() {
			JustBeforeEach(func() {
				err = boost.UpsertPod(context.TODO(), pod)
				Expect(err).ShouldNot(HaveOccurred())
				err = boost.DeletePod(context.TODO(), pod)
			})
			It("removes stored pod", func() {
				_, found := boost.Pod(pod.Name)
				Expect(found).To(BeFalse())
			})
		})
	})
})
