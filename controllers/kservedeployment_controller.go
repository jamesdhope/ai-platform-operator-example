package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/jamesdhope/ai-platform/api/v1alpha1"
)

// KServeDeploymentReconciler reconciles a KServeDeployment object
type KServeDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.ai-platform.io,resources=kservedeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.ai-platform.io,resources=kservedeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.ai-platform.io,resources=kservedeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete

func (r *KServeDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the KServeDeployment instance
	kserveDeployment := &platformv1alpha1.KServeDeployment{}
	if err := r.Get(ctx, req.NamespacedName, kserveDeployment); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("KServeDeployment resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get KServeDeployment")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling KServeDeployment", "name", kserveDeployment.Name, "version", kserveDeployment.Spec.Version)

	// Update status to Installing if not already set
	if kserveDeployment.Status.Phase == "" {
		if _, err := r.updateStatus(ctx, kserveDeployment, "Installing", "", nil); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Deploy KServe components
	installedComponents := []string{}

	// Deploy each requested component
	for _, component := range kserveDeployment.Spec.Components {
		logger.Info("Deploying component", "component", component)
		
		if err := r.deployComponent(ctx, kserveDeployment, component); err != nil {
			logger.Error(err, "Failed to deploy component", "component", component)
			return r.updateStatus(ctx, kserveDeployment, "Failed", "", installedComponents)
		}
		
		installedComponents = append(installedComponents, component)
	}

	// Update status to Ready
	return r.updateStatus(ctx, kserveDeployment, "Ready", kserveDeployment.Spec.Version, installedComponents)
}

func (r *KServeDeploymentReconciler) deployComponent(ctx context.Context, kd *platformv1alpha1.KServeDeployment, component string) error {
	logger := log.FromContext(ctx)
	
	switch component {
	case "kserve":
		return r.deployKServe(ctx, kd)
	case "cert-manager":
		return r.deployCertManager(ctx, kd)
	default:
		logger.Info("Unknown component, skipping", "component", component)
		return nil
	}
}

func (r *KServeDeploymentReconciler) deployKServe(ctx context.Context, kd *platformv1alpha1.KServeDeployment) error {
	logger := log.FromContext(ctx)
	logger.Info("Deploying KServe", "version", kd.Spec.Version)
	
	manifestURL := fmt.Sprintf("https://github.com/kserve/kserve/releases/download/%s/kserve.yaml", kd.Spec.Version)
	logger.Info("Applying KServe manifests", "url", manifestURL)
	
	// Use kubectl to apply the manifests
	// In a production operator, you'd parse YAML and use the Kubernetes API client
	// For this prototype, we'll use kubectl which is simpler
	if err := r.applyManifestURL(ctx, manifestURL); err != nil {
		logger.Error(err, "Failed to apply KServe manifests")
		return err
	}
	
	logger.Info("KServe manifests applied successfully")
	
	// Apply RawDeployment mode configuration
	logger.Info("Configuring KServe for RawDeployment mode")
	if err := r.configureRawDeployment(ctx); err != nil {
		logger.Error(err, "Failed to configure RawDeployment mode")
		return err
	}
	
	logger.Info("KServe configured for RawDeployment mode")
	
	// Deploy the inference service
	logger.Info("Deploying inference service")
	if err := r.deployInferenceService(ctx); err != nil {
		logger.Error(err, "Failed to deploy inference service")
		return err
	}
	
	logger.Info("Inference service deployed successfully")
	return nil
}

func (r *KServeDeploymentReconciler) deployCertManager(ctx context.Context, kd *platformv1alpha1.KServeDeployment) error {
	logger := log.FromContext(ctx)
	logger.Info("Deploying cert-manager")
	
	manifestURL := "https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml"
	logger.Info("Applying cert-manager manifests", "url", manifestURL)
	
	if err := r.applyManifestURL(ctx, manifestURL); err != nil {
		logger.Error(err, "Failed to apply cert-manager manifests")
		return err
	}
	
	logger.Info("cert-manager manifests applied successfully")
	return nil
}

func (r *KServeDeploymentReconciler) applyManifestURL(ctx context.Context, url string) error {
	logger := log.FromContext(ctx)
	
	// Fetch the manifest from URL
	logger.Info("Fetching manifest", "url", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch manifest: status %d", resp.StatusCode)
	}
	
	// Read the entire response
	manifestBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}
	
	// Split YAML documents and apply each one
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifestBytes), 4096)
	for {
		var obj unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			logger.Info("Skipping invalid YAML document", "error", err)
			continue
		}
		
		if obj.Object == nil {
			continue
		}
		
		logger.Info("Applying resource", 
			"kind", obj.GetKind(), 
			"name", obj.GetName(), 
			"namespace", obj.GetNamespace())
		
		// Try to create the resource
		if err := r.Create(ctx, &obj); err != nil {
			if errors.IsAlreadyExists(err) {
				// Don't update ConfigMaps - they may have been customized
				if obj.GetKind() == "ConfigMap" {
					logger.Info("ConfigMap already exists, skipping update", "name", obj.GetName(), "namespace", obj.GetNamespace())
				} else {
					logger.Info("Resource already exists, updating", "kind", obj.GetKind(), "name", obj.GetName())
					// Update the resource
					if err := r.Update(ctx, &obj); err != nil {
						logger.Error(err, "Failed to update resource", "kind", obj.GetKind(), "name", obj.GetName())
						// Continue with other resources even if one fails
					}
				}
			} else {
				logger.Error(err, "Failed to create resource", "kind", obj.GetKind(), "name", obj.GetName())
				// Continue with other resources
			}
		}
	}
	
	logger.Info("Finished applying manifests from URL")
	return nil
}

func (r *KServeDeploymentReconciler) applyManifestFile(ctx context.Context, path string) error {
	logger := log.FromContext(ctx)
	
	// Read the manifest file
	logger.Info("Reading manifest file", "path", path)
	manifestBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}
	
	// Split YAML documents and apply each one
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifestBytes), 4096)
	for {
		var obj unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			logger.Info("Skipping invalid YAML document", "error", err)
			continue
		}
		
		if obj.Object == nil {
			continue
		}
		
		logger.Info("Applying resource", 
			"kind", obj.GetKind(), 
			"name", obj.GetName(), 
			"namespace", obj.GetNamespace())
		
		// Try to create the resource
		if err := r.Create(ctx, &obj); err != nil {
			if errors.IsAlreadyExists(err) {
				logger.Info("Resource already exists, updating", "kind", obj.GetKind(), "name", obj.GetName())
				
				// Get the existing resource
				existing := &unstructured.Unstructured{}
				existing.SetGroupVersionKind(obj.GroupVersionKind())
				key := client.ObjectKey{
					Namespace: obj.GetNamespace(),
					Name:      obj.GetName(),
				}
				
				if err := r.Get(ctx, key, existing); err != nil {
					logger.Error(err, "Failed to get existing resource", "kind", obj.GetKind(), "name", obj.GetName())
					continue
				}
				
				// Update the resource
				obj.SetResourceVersion(existing.GetResourceVersion())
				if err := r.Update(ctx, &obj); err != nil {
					logger.Error(err, "Failed to update resource", "kind", obj.GetKind(), "name", obj.GetName())
				}
			} else {
				logger.Error(err, "Failed to create resource", "kind", obj.GetKind(), "name", obj.GetName())
			}
		}
	}
	
	logger.Info("Finished applying manifests from file")
	return nil
}

func (r *KServeDeploymentReconciler) configureRawDeployment(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("Applying RawDeployment configuration patch")
	
	// Apply the RawDeployment patch
	patchPath := "config/kserve-rawdeployment-patch.yaml"
	if err := r.applyManifestFile(ctx, patchPath); err != nil {
		logger.Error(err, "Failed to apply RawDeployment patch")
		return err
	}
	
	logger.Info("RawDeployment patch applied successfully")
	return nil
}

func (r *KServeDeploymentReconciler) deployInferenceService(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("Deploying InferenceService from manifest")
	
	// Apply the InferenceService manifest
	manifestPath := "config/samples/gemma2-inferenceservice.yaml"
	if err := r.applyManifestFile(ctx, manifestPath); err != nil {
		logger.Error(err, "Failed to apply InferenceService manifest")
		return err
	}
	
	logger.Info("InferenceService manifest applied successfully")
	return nil
}

func (r *KServeDeploymentReconciler) execCommand(cmd string) (string, error) {
	// This function is no longer needed but kept for compatibility
	return "Command execution not used", nil
}

func (r *KServeDeploymentReconciler) updateStatus(ctx context.Context, kd *platformv1alpha1.KServeDeployment, phase, version string, components []string) (ctrl.Result, error) {
	kd.Status.Phase = phase
	kd.Status.InstalledVersion = version
	kd.Status.InstalledComponents = components
	kd.Status.LastUpdated = metav1.Now()

	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: kd.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             phase,
		Message:            fmt.Sprintf("KServe deployment is %s", phase),
	}

	if phase == "Failed" {
		condition.Status = metav1.ConditionFalse
		condition.Message = "KServe deployment failed"
	}

	kd.Status.Conditions = []metav1.Condition{condition}

	if err := r.Status().Update(ctx, kd); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KServeDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.KServeDeployment{}).
		Complete(r)
}
