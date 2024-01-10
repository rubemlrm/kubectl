/*
Copyright 2021 The Kubernetes Authors.

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

package create

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	resource_requests "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreatePersistentVolumeValidation(t *testing.T) {
	pvcName := "pvc-testing"
	tests := map[string]struct {
		storageRequest string
		name           string
		expected       string
		accessModes    string
	}{
		"empty storage request": {
			name:           pvcName,
			storageRequest: "",
			accessModes:    "",
			expected:       "storage-request must be specified",
		},

		"empty name": {
			name:           "",
			storageRequest: "5Gi",
			accessModes:    "",
			expected:       "name must be specified",
		},

		"wrong access mode type": {
			name:           pvcName,
			storageRequest: "5Gi",
			accessModes:    "ReadWriteBoth",
			expected:       "provided access mode ReadWriteBoth is invalid",
		},
		"no error": {
			name:           pvcName,
			storageRequest: "5Gi",
			accessModes:    "ReadWriteOnce",
			expected:       "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			o := &CreatePersistentVolumeClaimOptions{
				StorageRequest: tc.storageRequest,
				Name:           tc.name,
				AccessModes:    tc.accessModes,
			}

			err := o.Validate()
			if err != nil && !strings.Contains(err.Error(), tc.expected) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCreatePersistentVolume(t *testing.T) {
	pvcName := "test-pvc"
	pvcStorageClassName := "test-class"
	tests := map[string]struct {
		storageRequest   string
		storageLimit     string
		accessModes      string
		name             string
		storageClassName string
		expected         *corev1.PersistentVolumeClaim
		err              error
	}{
		"just storage request": {
			storageRequest:   "5Gi",
			storageLimit:     "",
			accessModes:      "",
			name:             pvcName,
			storageClassName: "",
			err:              nil,
			expected: &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "PersistentVolumeClaim"},
				ObjectMeta: metav1.ObjectMeta{
					Name: pvcName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource_requests.MustParse("5Gi"),
						},
					},
				},
			},
		},
		"storage request and storageClassName": {
			storageRequest:   "5Gi",
			storageLimit:     "",
			accessModes:      "",
			name:             pvcName,
			storageClassName: pvcStorageClassName,
			err:              nil,
			expected: &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "PersistentVolumeClaim"},
				ObjectMeta: metav1.ObjectMeta{
					Name: pvcName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &pvcStorageClassName,
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource_requests.MustParse("5Gi"),
						},
					},
				},
			},
		},
		"storage request and limits": {
			storageRequest:   "5Gi",
			storageLimit:     "10Gi",
			accessModes:      "",
			name:             pvcName,
			storageClassName: "",
			err:              nil,
			expected: &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "PersistentVolumeClaim"},
				ObjectMeta: metav1.ObjectMeta{
					Name: pvcName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource_requests.MustParse("5Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceStorage: resource_requests.MustParse("10Gi"),
						},
					},
				},
			},
		},
		"storage request and access modes": {
			storageRequest:   "5Gi",
			storageLimit:     "",
			accessModes:      "ReadWriteOnce",
			name:             pvcName,
			storageClassName: "",
			err:              nil,
			expected: &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "PersistentVolumeClaim"},
				ObjectMeta: metav1.ObjectMeta{
					Name: pvcName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource_requests.MustParse("5Gi"),
						},
					},
					AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
				},
			},
		},

		"storage request can't be higher or equal with storage limit": {
			storageRequest:   "5Gi",
			storageLimit:     "5Gi",
			accessModes:      "ReadWriteOnce",
			name:             pvcName,
			storageClassName: "",
			err:              fmt.Errorf("Resource limit is the same/less than the resource request"),
			expected:         nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			o := &CreatePersistentVolumeClaimOptions{
				Name:             tc.name,
				StorageRequest:   tc.storageRequest,
				StorageLimit:     tc.storageLimit,
				AccessModes:      tc.accessModes,
				StorageClassName: tc.storageClassName,
			}
			pvc, err := o.createPersistentVolumeClaim()
			if !apiequality.Semantic.DeepEqual(pvc, tc.expected) {
				t.Errorf("expected:\n%#v\ngot:\n%#v", tc.expected, pvc)
			}
			if tc.err != nil {
				if err.Error() != tc.err.Error() {
					t.Errorf("expected:\n%#v\ngot:\n%#v", tc.err, err)
				}
			}

		})
	}
}
