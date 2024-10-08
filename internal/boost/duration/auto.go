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

package duration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	v1 "k8s.io/api/core/v1"
)

const (
	AutoDurationPolicyName = "AutoDuration"
)

type RequestPayload struct {
	PodName      string `json:"podName"`
	PodNamespace string `json:"podNamespace"`
}

type AutoDurationPolicy struct {
	apiEndpoint string
}

type DurationPrediction struct {
	Duration string `json:"duration"`
}

func (p *AutoDurationPolicy) Name() string {
	return AutoDurationPolicyName
}

// Valid returns true if the pod is still within the duration
func (p *AutoDurationPolicy) Valid(pod *v1.Pod) bool {
	fmt.Printf("Pod in Duration policy: %v\n", pod)
	now := time.Now()

	duration, err := p.GetDuration(pod)

	if err != nil {
		log.Printf("error getting duration: %v", err)
		return false
	}

	return pod.CreationTimestamp.Add(duration).After(now)
}

func NewAutoDurationPolicy(apiEndpoint string) *AutoDurationPolicy {
	return &AutoDurationPolicy{
		apiEndpoint: apiEndpoint,
	}
}

func (p *AutoDurationPolicy) GetDuration(pod *v1.Pod) (time.Duration, error) {
	prediction, err := p.getPrediction(pod)
	if err != nil {
		log.Printf("error getting prediction: %v", err)
		return 0, err
	}
	return time.ParseDuration(prediction.Duration)
}

func (p *AutoDurationPolicy) getPrediction(pod *v1.Pod) (*DurationPrediction, error) {

	imageName := pod.Spec.Containers[0].Image

	log.Printf("getting predicted duration for image: %s", imageName)

	req, err := http.NewRequest("GET", p.apiEndpoint+"/duration", nil)
	if err != nil {
		log.Printf("error creating request: %v", err)
		return nil, err
	}

	q := req.URL.Query()
	q.Add("imageName", imageName)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("error sending request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var prediction DurationPrediction
	if err := json.NewDecoder(resp.Body).Decode(&prediction); err != nil {
		log.Printf("error decoding response: %v", err)
		return nil, err
	}

	log.Printf("predicted duration: %s", prediction.Duration)

	return &prediction, nil
}

func (p *AutoDurationPolicy) NotifyReversion(pod *v1.Pod) error {

	podName := pod.Name
	podNamespace := pod.Namespace

	podData, err := json.Marshal(RequestPayload{
		PodName:      podName,
		PodNamespace: podNamespace,
	})

	if err != nil {
		return err
	}

	resp, err := http.Post(p.apiEndpoint+"/notify", "application/json", bytes.NewBuffer(podData))
	if err != nil {
		log.Printf("error sending notify request: %v", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("unexpected notify status code: %d", resp.StatusCode)
	}

	return nil
}
