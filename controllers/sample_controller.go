/*
Copyright 2022.

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

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	examplecomv1 "example.com/sample-operator/api/v1"
)

// SampleReconciler reconciles a Sample object
type SampleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=example.com.example.com,resources=samples,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=example.com.example.com,resources=samples/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=example.com.example.com,resources=samples/finalizers,verbs=update

func (r *SampleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	sample := examplecomv1.Sample{}
	deployment := apps.Deployment{}

	// Get the current Sample resource
	if err := r.Client.Get(ctx, req.NamespacedName, &sample); err != nil {
		logger.Error(err, "Failed to get resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get the deployment
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: sample.Namespace, Name: sample.Name}, &deployment)

	// Create the deployment if it does not exist
	if apierrors.IsNotFound(err) {
		deployment = *createDeployment(sample)

		if err := r.Client.Create(ctx, &deployment); err != nil {
			logger.Error(err, "Failed to create resource")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if err != nil {
		logger.Error(err, "Faild to get Deployment for resource")
		return ctrl.Result{}, err
	}

	// Rescale the deployment
	if *deployment.Spec.Replicas != *sample.Spec.Replicas {
		deployment.Spec.Replicas = sample.Spec.Replicas

		if err := r.Client.Update(ctx, &deployment); err != nil {
			logger.Error(err, "Patching resource failed")
			return ctrl.Result{}, err
		}
	}

	// Get the selector from the deployment
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)

	if err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Update the status of the Sample Resource
	sample.Status = examplecomv1.SampleStatus{
		Replicas: deployment.Status.Replicas,
		Selector: selector.String(),
	}

	if err := r.Status().Update(ctx, &sample); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func createDeployment(sample examplecomv1.Sample) *apps.Deployment {
	deployment := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: sample.Name,
			Namespace: sample.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&sample, examplecomv1.GroupVersion.WithKind("Sample"))},
		},
		Spec: apps.DeploymentSpec{
			Replicas: sample.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example.com/sample": sample.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"example.com/sample": sample.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Image: "bash",
							Command: []string{
								"sleep",
								"10000",
							},
						},
					},
				},
			},
		},
	}

	return &deployment
}

// SetupWithManager sets up the controller with the Manager.
func (r *SampleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&examplecomv1.Sample{}).
		Complete(r)
}
