/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "kv-operator/api/v1alpha1"
)

type KVClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=storage.mydatabase.io,resources=kvclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.mydatabase.io,resources=kvclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storage.mydatabase.io,resources=kvclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *KVClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var kvCluster storagev1alpha1.KVCluster
	if err := r.Get(ctx, req.NamespacedName, &kvCluster); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	svcName := kvCluster.Name + "-service"
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: kvCluster.Namespace}, svc)
	if err != nil && errors.IsNotFound(err) {
		newSvc := r.serviceForKVCluster(&kvCluster)
		logger.Info("Creating a new Headless Service", "Namespace", newSvc.Namespace, "Name", newSvc.Name)
		if err = r.Create(ctx, newSvc); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	stsName := kvCluster.Name
	sts := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: stsName, Namespace: kvCluster.Namespace}, sts)
	if err != nil && errors.IsNotFound(err) {
		newSts := r.statefulSetForKVCluster(&kvCluster)
		logger.Info("Creating a new StatefulSet", "Namespace", newSts.Namespace, "Name", newSts.Name)
		if err = r.Create(ctx, newSts); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		size := kvCluster.Spec.Size
		if *sts.Spec.Replicas != size {
			logger.Info("Updating StatefulSet Replicas", "Old", *sts.Spec.Replicas, "New", size)
			sts.Spec.Replicas = &size
			if err = r.Update(ctx, sts); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *KVClusterReconciler) serviceForKVCluster(kvc *storagev1alpha1.KVCluster) *corev1.Service {
	labels := map[string]string{"app": kvc.Name}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvc.Name + "-service",
			Namespace: kvc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{Name: "grpc", Port: 50051},
				{Name: "raft", Port: 60051},
				{Name: "metrics", Port: 51051},
			},
		},
	}
	controllerutil.SetControllerReference(kvc, svc, r.Scheme)
	return svc
}

func (r *KVClusterReconciler) statefulSetForKVCluster(kvc *storagev1alpha1.KVCluster) *appsv1.StatefulSet {
	labels := map[string]string{"app": kvc.Name}
	replicas := kvc.Spec.Size

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvc.Name,
			Namespace: kvc.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: kvc.Name + "-service",
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           "kv-storage:latest",
						Name:            "storage-node",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env: []corev1.EnvVar{
							{
								Name: "POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.name",
									},
								},
							},
							{
								Name:  "NODE_ID",
								Value: "$(POD_NAME)",
							},
							{
								Name:  "SEED_NODE_ID",
								Value: kvc.Name + "-0",
							},
							{
								Name:  "SEED_NODE_ADDR",
								Value: fmt.Sprintf("%s-0.%s-service.%s.svc.cluster.local:50051", kvc.Name, kvc.Name, kvc.Namespace),
							},
						},
						Ports: []corev1.ContainerPort{
							{ContainerPort: 50051, Name: "grpc"},
							{ContainerPort: 60051, Name: "raft"},
							{ContainerPort: 51051, Name: "metrics"},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "data-volume",
							MountPath: "/app/raft-data",
						}},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "data-volume",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			}},
		},
	}
	_ = controllerutil.SetControllerReference(kvc, sts, r.Scheme)
	return sts
}

func (r *KVClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.KVCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
