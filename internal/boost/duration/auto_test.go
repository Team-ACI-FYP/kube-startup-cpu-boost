package duration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"

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

		fmt.Fprintln(GinkgoWriter, "queryParams:", queryParams)

		imageName := queryParams.Get("imageName")

		assert.Equal(t, "test-image", imageName)

		prediction := DurationPrediction{
			Duration: "5m0s",
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

	parcedPrediction, err := time.ParseDuration(prediction.Duration)
	assert.NoError(t, err)
	assert.Equal(t, 5*time.Minute, parcedPrediction)
}

func TestAutoDurationPolicy_IsValid(t *testing.T) {

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/duration", r.URL.Path)

		queryParams := r.URL.Query()

		fmt.Fprintln(GinkgoWriter, "queryParams:", queryParams)

		imageName := queryParams.Get("imageName")

		assert.Equal(t, "valid-image", imageName)

		prediction := DurationPrediction{
			Duration: "5m0s",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prediction)
	}))
	defer mockServer.Close()

	// Create an instance of AutoDurationPolicy
	policy := NewAutoDurationPolicy(mockServer.URL)

	// Create a sample pod with a valid image name
	validPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "valid-pod",
			Namespace:         "test-namespace",
			CreationTimestamp: metav1.Time{Time: time.Now()},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "valid-image",
				},
			},
		},
	}

	// Create a sample pod with an invalid image name
	invalidPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "invalid-pod",
			Namespace:         "test-namespace",
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-5*time.Minute - 1*time.Microsecond)},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "valid-image",
				},
			},
		},
	}

	duration, err := policy.GetDuration(validPod)
	assert.NoError(t, err)
	assert.Equal(t, 5*time.Minute, duration)

	// Check if the valid pod is valid
	isValid := policy.Valid(validPod)
	assert.True(t, isValid)

	isValid = policy.Valid(validPod)
	assert.True(t, isValid)

	isValid = policy.Valid(validPod)
	assert.True(t, isValid)

	isValid = policy.Valid(validPod)
	assert.True(t, isValid)

	// Check if the invalid pod is invalid
	isValid = policy.Valid(invalidPod)
	assert.False(t, isValid)
}
