/*
Copyright 2023.

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

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ingv1 "github.com/wrhb123/ingress-manager/api/v1"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ing.igtest.com,resources=apps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ing.igtest.com,resources=apps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ing.igtest.com,resources=apps/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the App object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	app := &ingv1.App{}

	// 从缓存中取app
	err := r.Get(ctx, req.NamespacedName, app)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// deployment
	deployment := NewDeployment(app)
	err = controllerutil.SetControllerReference(app, deployment, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	d := &v1.Deployment{}
	err = r.Get(ctx, req.NamespacedName, d)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = r.Create(ctx, deployment); err != nil {
				logger.Error(err, "create deployment failed")
				return ctrl.Result{}, err
			}
		}
	}

	if app.Spec.Image != deployment.Spec.Template.Spec.Containers[0].Image {
		err = r.Update(ctx, deployment)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// service
	service := NewService(app)
	err = controllerutil.SetControllerReference(app, service, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	s := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, s)
	if err != nil {
		if errors.IsNotFound(err) && app.Spec.EnableSerivce {
			if err = r.Create(ctx, service); err != nil {
				logger.Error(err, "create service failed")
				return ctrl.Result{}, err
			}
		}
		if !errors.IsNotFound(err) && app.Spec.EnableSerivce {
			return ctrl.Result{}, err
		}
	}

	if app.Spec.EnableSerivce {
		logger.Info("skip update")
	} else {
		err = r.Delete(ctx, s)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// ingress
	ingress := NewIngress(app)
	err = controllerutil.SetControllerReference(app, ingress, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	i := &netv1.Ingress{}
	err = r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, i)
	if err != nil {
		if errors.IsNotFound(err) && app.Spec.EnableIngress {
			if err = r.Create(ctx, ingress); err != nil {
				logger.Error(err, "create ingress failed")
				return ctrl.Result{}, err
			}
		}
		if !errors.IsNotFound(err) && app.Spec.EnableIngress {
			return ctrl.Result{}, err
		}
	}

	if app.Spec.EnableIngress {
		logger.Info("skip update")
	} else {
		err = r.Delete(ctx, i)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ingv1.App{}).
		Owns(&netv1.Ingress{}).
		Owns(&corev1.Service{}).
		Owns(&v1.Deployment{}).
		Complete(r)
}
