package duration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAutoDurationPolicy_GetDuration(t *testing.T) {
	// Mock API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/duration", r.URL.Path)

		queryParams := r.URL.Query()
		podName := queryParams.Get("imageName")

		assert.Equal(t, "test-image", podName)

		prediction := DurationPrediction{
			Duration: "5m",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prediction)
	}))
	defer mockServer.Close()

	// Create an instance of AutoDurationPolicy with the mock server URL
	policy := NewAutoDurationPolicy(mockServer.URL)

	// Create a sample pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}

	// Call the GetDuration method
	duration, err := policy.GetDuration(pod)
	assert.NoError(t, err)
	assert.Equal(t, 5*time.Minute, duration)
}

func TestAutoDurationPolicy_getPrediction(t *testing.T) {
	// Mock API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/duration", r.URL.Path)

		queryParams := r.URL.Query()

		fmt.Println(queryParams)
		imageName := queryParams.Get("imageName")

		assert.Equal(t, "test-image", imageName)

		prediction := DurationPrediction{
			Duration: "5m",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prediction)
	}))
	defer mockServer.Close()

	// Create an instance of AutoDurationPolicy with the mock server URL
	policy := NewAutoDurationPolicy(mockServer.URL)

	// Create a sample pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}

	// Call the getPrediction method
	prediction, err := policy.getPrediction(pod)
	assert.NoError(t, err)
	assert.Equal(t, "5m", prediction.Duration)
}
