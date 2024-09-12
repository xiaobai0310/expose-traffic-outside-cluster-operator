/*
Copyright 2024.

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
	exposetrafficoutsideclusterv1alpha1 "github.com/xiaobai0310/expose-traffic-outside-cluster-operator/api/v1alpha1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ExposeAppReconciler reconciles a ExposeApp object
type ExposeAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=expose-traffic-outside-cluster.bailu.io,resources=exposeapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=expose-traffic-outside-cluster.bailu.io,resources=exposeapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=expose-traffic-outside-cluster.bailu.io,resources=exposeapps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ExposeApp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *ExposeAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	// Fetch the ExposeApp instance
	app := &exposetrafficoutsideclusterv1alpha1.ExposeApp{}
	// get app object, if not found, ignore
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// deploy processing
	// create the deployment object
	deployObj := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    app.Labels,
		},
		Spec: appv1.DeploymentSpec{
			Replicas: &app.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: app.ObjectMeta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: app.ObjectMeta.Labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  app.Name,
							Image: app.Spec.Image,
							Ports: app.Spec.Ports,
						},
					},
				},
			},
		},
	}

	// set the controller reference, so that the app object can manage the deployment object
	if err := controllerutil.SetControllerReference(app, deployObj, r.Scheme); err != nil {
		klog.Warningf("SetControllerReference error with deployment: %v", err)
		return ctrl.Result{}, err
	}

	// check if the deployment object exists, if not, create it, if exists, update it
	d := &appv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, d); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, deployObj); err != nil {
				klog.Errorf("Create deployment error: %v", err)
				return ctrl.Result{}, err
			}
		}
	} else {
		if err := r.Update(ctx, deployObj); err != nil {
			klog.Errorf("Update deployment error: %v", err)
			return ctrl.Result{}, err
		}
	}

	// service processing
	// create the service object
	serviceObj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    app.Labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": app.Name,
			},
			Type:  corev1.ServiceTypeClusterIP,
			Ports: app.Spec.ServicePorts,
		},
	}

	// set the controller reference, so that the app object can manage the service object
	if err := controllerutil.SetControllerReference(app, serviceObj, r.Scheme); err != nil {
		klog.Warningf("SetControllerReference error with service: %v", err)
		return ctrl.Result{}, err
	}

	// check if the service object exists, if not, create it, if exists, update it
	s := &corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, s); err != nil {
		if errors.IsNotFound(err) && app.Spec.EnableService {
			if err := r.Create(ctx, serviceObj); err != nil {
				klog.Errorf("Create service error: %v", err)
				return ctrl.Result{}, err
			}
		}
	} else {
		if app.Spec.EnableService {
			if err := r.Update(ctx, serviceObj); err != nil {
				klog.Errorf("EnableService is true and service is exist，Update service error: %v", err)
				return ctrl.Result{}, err
			} else {
				if err := r.Delete(ctx, s); err != nil {
					klog.Errorf("EnableService is false, Delete service error: %v", err)
					return ctrl.Result{}, err
				}
			}
		}
	}

	// ingress processing
	// todo: 使用adminssion检验，如果启用了Ingress，就必须启用Service
	// todo: 使用adminssion设置默认值
	if !app.Spec.EnableIngress {
		return ctrl.Result{}, nil
	}

	// create the ingress object
	ingressObj := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    app.Labels,
		},
		Spec: netv1.IngressSpec{
			Rules: app.Spec.IngressPorts,
		},
	}

	// set the controller reference, so that the app object can manage the ingress object
	if err := controllerutil.SetControllerReference(app, ingressObj, r.Scheme); err != nil {
		klog.Warningf("SetControllerReference error with ingress: %v", err)
		return ctrl.Result{}, err
	}

	// check if the ingress object exists, if not, create it, if exists, update it
	i := &netv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, i); err != nil {
		if errors.IsNotFound(err) && app.Spec.EnableIngress {
			if err := r.Create(ctx, ingressObj); err != nil {
				klog.Errorf("EnableIngress is true and ingress is not exist, Create ingress failed, error: %v", err)
				return ctrl.Result{}, err
			}
		}
	} else {
		if app.Spec.EnableIngress {
			if err := r.Update(ctx, ingressObj); err != nil {
				klog.Errorf("EnableIngress is true and ingress is exist, Update ingress failed, error: %v", err)
			}
		} else {
			if err := r.Delete(ctx, i); err != nil {
				klog.Errorf("EnableIngress is false, Delete ingress failed, error: %v", err)
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExposeAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&exposetrafficoutsideclusterv1alpha1.ExposeApp{}).
		// 设置以后，如果Deployment、Service、Ingress发生变化以后，会触发Reconcile
		Owns(&appv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&netv1.Ingress{}).
		Complete(r)
}
