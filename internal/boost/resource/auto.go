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

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiResource "k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ContextKey string

type AutoPolicy struct {
	apiEndpoint string
}

type ResourcePrediction struct {
	CPURequests string `json:"cpuRequests"`
	CPULimits   string `json:"cpuLimits"`
}

type RequestPayload struct {
	PodName      string `json:"podName"`
	PodNamespace string `json:"podNamespace"`
}

func NewAutoPolicy(apiEndpoint string) ContainerPolicy {
	return &AutoPolicy{
		apiEndpoint: apiEndpoint,
	}
}

func (p *AutoPolicy) NewResources(ctx context.Context, container *corev1.Container) *corev1.ResourceRequirements {
	log := ctrl.LoggerFrom(ctx).WithName("auto-cpu-policy")
	prediction, err := p.getPrediction(container)
	if prediction == nil {
		log.Error(err, "failed to get prediction")
		return nil
	}

	if err != nil {
		log.Error(err, "failed to get prediction")
		return nil
	}

	cpuRequests, err := apiResource.ParseQuantity(prediction.CPURequests)
	if err != nil {
		log.Error(err, "failed to parse CPU requests")
		return nil
	}
	cpuLimits, err := apiResource.ParseQuantity(prediction.CPULimits)
	if err != nil {
		log.Error(err, "failed to parse CPU limits")
		return nil
	}

	log = log.WithValues("newCPURequests", cpuRequests.String(), "newCPULimits", cpuLimits.String())
	result := container.Resources.DeepCopy()
	p.setResource(corev1.ResourceCPU, result.Requests, cpuRequests, log)
	p.setResource(corev1.ResourceCPU, result.Limits, cpuLimits, log)

	fmt.Printf("result: %+v\n", result)
	return result
}

func (p *AutoPolicy) setResource(resource corev1.ResourceName, resources corev1.ResourceList, target apiResource.Quantity, log logr.Logger) {
	if target.IsZero() {
		return
	}
	current, ok := resources[resource]
	if !ok {
		return
	}
	if target.Cmp(current) < 0 {
		log.V(2).Info("container has higher CPU requests than policy")
		return
	}
	resources[resource] = target
}

func (p *AutoPolicy) getPrediction(container *corev1.Container) (*ResourcePrediction, error) {

	// Retrieve the pod information from the context
	imageName := container.Image

	fmt.Println("Image Name From ctx : ", imageName)

	if imageName == "" {
		fmt.Println("image name is empty")
		return nil, fmt.Errorf("image name is empty")
	}

	fmt.Printf("apiEndpoint: %+v\n", p.apiEndpoint)

	// Create a new HTTP request with the pod information
	req, err := http.NewRequest("GET", p.apiEndpoint+"/cpu", nil)
	if err != nil {
		fmt.Println("failed to create request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("imageName", imageName)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("failed to send request")
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for a successful response status code
	if resp.StatusCode != http.StatusOK {
		fmt.Println("unexpected status code")
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	fmt.Printf("resp: %+v\n", resp)

	// Decode the response body into a ResourcePrediction struct
	var prediction ResourcePrediction
	if err := json.NewDecoder(resp.Body).Decode(&prediction); err != nil {
		fmt.Println("failed to decode response")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &prediction, nil
}
