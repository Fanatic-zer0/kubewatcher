package watcher

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"k8watch/internal/diff"
	"k8watch/internal/notifier"
	"k8watch/internal/storage"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type Watcher struct {
	clientset *kubernetes.Clientset
	storage   *storage.Storage
	notifier  *notifier.SlackNotifier
	stopCh    chan struct{}
}

// NewWatcher creates a new Kubernetes watcher
func NewWatcher(kubeconfig string, storage *storage.Storage, slackWebhook string) (*Watcher, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	slackNotifier := notifier.NewSlackNotifier(slackWebhook)
	if slackNotifier.IsEnabled() {
		log.Println("Slack notifications enabled")
		// Test connection
		if err := slackNotifier.TestConnection(); err != nil {
			log.Printf("Warning: Failed to send test Slack message: %v", err)
		}
	}

	return &Watcher{
		clientset: clientset,
		storage:   storage,
		notifier:  slackNotifier,
		stopCh:    make(chan struct{}),
	}, nil
}

// Start starts watching all resources
func (w *Watcher) Start() error {
	log.Println("Starting watchers...")

	// Start deployment watcher
	go w.watchDeployments()

	// Start configmap watcher
	go w.watchConfigMaps()

	// Start secret watcher
	go w.watchSecrets()

	// Start service watcher
	go w.watchServices()

	// Start ingress watcher
	go w.watchIngresses()

	// Start statefulset watcher
	go w.watchStatefulSets()

	// Start daemonset watcher
	go w.watchDaemonSets()

	// Start cronjob watcher
	go w.watchCronJobs()

	// Start job watcher
	go w.watchJobs()

	log.Println("All watchers started successfully")
	return nil
}

// Stop stops all watchers
func (w *Watcher) Stop() {
	close(w.stopCh)
	log.Println("Stopped all watchers")
}

// watchDeployments watches deployment changes
func (w *Watcher) watchDeployments() {
	watchlist := cache.NewListWatchFromClient(
		w.clientset.AppsV1().RESTClient(),
		"deployments",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&appsv1.Deployment{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				w.handleDeploymentEvent(watch.Added, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.handleDeploymentEvent(watch.Modified, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				w.handleDeploymentEvent(watch.Deleted, obj, nil)
			},
		},
	)

	controller.Run(w.stopCh)
}

// handleDeploymentEvent processes deployment events
func (w *Watcher) handleDeploymentEvent(eventType watch.EventType, oldObj, newObj interface{}) {
	var deployment *appsv1.Deployment
	var oldDeployment *appsv1.Deployment

	if newObj != nil {
		deployment = newObj.(*appsv1.Deployment)
	}
	if oldObj != nil {
		oldDeployment = oldObj.(*appsv1.Deployment)
	}
	if deployment == nil && oldDeployment != nil {
		deployment = oldDeployment
	}

	// Skip system-generated namespaces
	if deployment.Namespace == "kube-system" || deployment.Namespace == "kube-public" || deployment.Namespace == "kube-node-lease" {
		return
	}

	// For MODIFIED events, only track meaningful changes
	if eventType == watch.Modified && oldDeployment != nil {
		hasChanges, changeDescription := w.detectMeaningfulChanges(oldDeployment, deployment)
		if !hasChanges {
			return // Skip this event
		}

		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: deployment.Namespace,
			Kind:      "Deployment",
			Name:      deployment.Name,
			Action:    string(eventType),
			Diff:      changeDescription,
		}

		// Extract images
		oldMap := convertToMap(oldDeployment)
		newMap := convertToMap(deployment)
		event.ImageBefore = diff.ExtractImage(oldMap)
		event.ImageAfter = diff.ExtractImage(newMap)

		// Extract metadata
		metadata := map[string]interface{}{
			"replicas": deployment.Spec.Replicas,
		}
		metadataJSON, _ := json.Marshal(metadata)
		event.Metadata = string(metadataJSON)

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving deployment event: %v", err)
		} else {
			log.Printf("Saved %s event for deployment %s/%s: %s", eventType, deployment.Namespace, deployment.Name, changeDescription)
		}
		return
	}

	// For ADDED/DELETED events, save them
	if eventType == watch.Added || eventType == watch.Deleted {
		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: deployment.Namespace,
			Kind:      "Deployment",
			Name:      deployment.Name,
			Action:    string(eventType),
		}

		if eventType == watch.Added {
			newMap := convertToMap(deployment)
			event.ImageAfter = diff.ExtractImage(newMap)
			event.Diff = "Deployment created"
		} else {
			event.Diff = "Deployment deleted"
		}

		metadata := map[string]interface{}{
			"replicas": deployment.Spec.Replicas,
		}
		metadataJSON, _ := json.Marshal(metadata)
		event.Metadata = string(metadataJSON)

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving deployment event: %v", err)
		} else {
			log.Printf("Saved %s event for deployment %s/%s", eventType, deployment.Namespace, deployment.Name)
		}
	}
}

// detectMeaningfulChanges checks for scale, image, or spec changes
func (w *Watcher) detectMeaningfulChanges(oldDep, newDep *appsv1.Deployment) (bool, string) {
	changes := []string{}

	// Check for replica changes (scale up/down)
	oldReplicas := int32(0)
	newReplicas := int32(0)
	if oldDep.Spec.Replicas != nil {
		oldReplicas = *oldDep.Spec.Replicas
	}
	if newDep.Spec.Replicas != nil {
		newReplicas = *newDep.Spec.Replicas
	}

	if oldReplicas != newReplicas {
		if newReplicas > oldReplicas {
			changes = append(changes, fmt.Sprintf("Scaled up: %d → %d replicas", oldReplicas, newReplicas))
		} else {
			changes = append(changes, fmt.Sprintf("Scaled down: %d → %d replicas", oldReplicas, newReplicas))
		}
	}

	// Check for image changes
	oldContainers := oldDep.Spec.Template.Spec.Containers
	newContainers := newDep.Spec.Template.Spec.Containers

	if len(oldContainers) > 0 && len(newContainers) > 0 {
		oldImage := oldContainers[0].Image
		newImage := newContainers[0].Image

		if oldImage != newImage {
			changes = append(changes, fmt.Sprintf("Image updated: %s → %s", oldImage, newImage))
		}

		// Check for resource changes
		oldResources := oldContainers[0].Resources
		newResources := newContainers[0].Resources

		if !oldResources.Limits.Cpu().Equal(*newResources.Limits.Cpu()) ||
			!oldResources.Limits.Memory().Equal(*newResources.Limits.Memory()) {
			changes = append(changes, "Resource limits updated")
		}

		if !oldResources.Requests.Cpu().Equal(*newResources.Requests.Cpu()) ||
			!oldResources.Requests.Memory().Equal(*newResources.Requests.Memory()) {
			changes = append(changes, "Resource requests updated")
		}

		// Check for env var changes
		if len(oldContainers[0].Env) != len(newContainers[0].Env) {
			changes = append(changes, "Environment variables updated")
		}
	}

	// Check for strategy changes
	if oldDep.Spec.Strategy.Type != newDep.Spec.Strategy.Type {
		changes = append(changes, fmt.Sprintf("Deployment strategy changed: %s → %s", oldDep.Spec.Strategy.Type, newDep.Spec.Strategy.Type))
	}

	if len(changes) == 0 {
		return false, ""
	}

	return true, fmt.Sprintf("%s", changes[0])
}

// watchConfigMaps watches configmap changes
func (w *Watcher) watchConfigMaps() {
	watchlist := cache.NewListWatchFromClient(
		w.clientset.CoreV1().RESTClient(),
		"configmaps",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&corev1.ConfigMap{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				w.handleConfigMapEvent(watch.Added, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.handleConfigMapEvent(watch.Modified, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				w.handleConfigMapEvent(watch.Deleted, obj, nil)
			},
		},
	)

	controller.Run(w.stopCh)
}

// handleConfigMapEvent processes configmap events
func (w *Watcher) handleConfigMapEvent(eventType watch.EventType, oldObj, newObj interface{}) {
	var cm *corev1.ConfigMap
	var oldCM *corev1.ConfigMap

	if newObj != nil {
		cm = newObj.(*corev1.ConfigMap)
	}
	if oldObj != nil {
		oldCM = oldObj.(*corev1.ConfigMap)
	}
	if cm == nil && oldCM != nil {
		cm = oldCM
	}

	// Skip system-generated namespaces
	if cm.Namespace == "kube-system" || cm.Namespace == "kube-public" || cm.Namespace == "kube-node-lease" {
		return
	}

	// For MODIFIED events, only track meaningful changes
	if eventType == watch.Modified && oldCM != nil {
		hasChanges, changeDescription := w.detectConfigMapChanges(oldCM, cm)
		if !hasChanges {
			return // Skip this event
		}

		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: cm.Namespace,
			Kind:      "ConfigMap",
			Name:      cm.Name,
			Action:    string(eventType),
			Diff:      changeDescription,
		}

		// Extract metadata
		keys := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			keys = append(keys, k)
		}
		metadata := map[string]interface{}{
			"keys": keys,
		}
		metadataJSON, _ := json.Marshal(metadata)
		event.Metadata = string(metadataJSON)

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving configmap event: %v", err)
		} else {
			log.Printf("Saved %s event for configmap %s/%s: %s", eventType, cm.Namespace, cm.Name, changeDescription)
		}
		return
	}

	// For ADDED/DELETED events, save them
	if eventType == watch.Added || eventType == watch.Deleted {
		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: cm.Namespace,
			Kind:      "ConfigMap",
			Name:      cm.Name,
			Action:    string(eventType),
		}

		if eventType == watch.Added {
			keys := make([]string, 0, len(cm.Data))
			for k := range cm.Data {
				keys = append(keys, k)
			}
			event.Diff = fmt.Sprintf("ConfigMap created with %d key(s)", len(keys))
		} else {
			event.Diff = "ConfigMap deleted"
		}

		keys := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			keys = append(keys, k)
		}
		metadata := map[string]interface{}{
			"keys": keys,
		}
		metadataJSON, _ := json.Marshal(metadata)
		event.Metadata = string(metadataJSON)

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving configmap event: %v", err)
		} else {
			log.Printf("Saved %s event for configmap %s/%s", eventType, cm.Namespace, cm.Name)
		}
	}
}

// detectConfigMapChanges checks for key additions, removals, or value changes
func (w *Watcher) detectConfigMapChanges(oldCM, newCM *corev1.ConfigMap) (bool, string) {
	oldKeys := make(map[string]bool)
	for k := range oldCM.Data {
		oldKeys[k] = true
	}

	newKeys := make(map[string]bool)
	for k := range newCM.Data {
		newKeys[k] = true
	}

	// Check for added keys
	addedKeys := []string{}
	for k := range newCM.Data {
		if !oldKeys[k] {
			addedKeys = append(addedKeys, k)
		}
	}

	// Check for removed keys
	removedKeys := []string{}
	for k := range oldCM.Data {
		if !newKeys[k] {
			removedKeys = append(removedKeys, k)
		}
	}

	// Check for modified values and collect full details
	modifiedKeys := []string{}
	detailedChanges := []string{}
	for k, newVal := range newCM.Data {
		if oldVal, exists := oldCM.Data[k]; exists && oldVal != newVal {
			modifiedKeys = append(modifiedKeys, k)
			// Store full change details for timeline
			detailedChanges = append(detailedChanges, fmt.Sprintf("[%s]\n- %s\n+ %s", k, oldVal, newVal))
		}
	}

	if len(addedKeys) == 0 && len(removedKeys) == 0 && len(modifiedKeys) == 0 {
		return false, ""
	}

	// Build detailed description (git diff style)
	var changeDesc string
	if len(addedKeys) > 0 {
		changeDesc = fmt.Sprintf("Keys added: %v", addedKeys)
	} else if len(removedKeys) > 0 {
		changeDesc = fmt.Sprintf("Keys removed: %v", removedKeys)
	} else if len(detailedChanges) > 0 {
		// Return full diff details
		changeDesc = "Keys modified: " + fmt.Sprintf("%v", modifiedKeys) + "\n\n" + strings.Join(detailedChanges, "\n\n")
	}

	return true, changeDesc
}

// watchSecrets watches secret changes
func (w *Watcher) watchSecrets() {
	watchlist := cache.NewListWatchFromClient(
		w.clientset.CoreV1().RESTClient(),
		"secrets",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&corev1.Secret{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				w.handleSecretEvent(watch.Added, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.handleSecretEvent(watch.Modified, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				w.handleSecretEvent(watch.Deleted, obj, nil)
			},
		},
	)

	controller.Run(w.stopCh)
}

// handleSecretEvent processes secret events
func (w *Watcher) handleSecretEvent(eventType watch.EventType, oldObj, newObj interface{}) {
	var secret *corev1.Secret
	var oldSecret *corev1.Secret

	if newObj != nil {
		secret = newObj.(*corev1.Secret)
	}
	if oldObj != nil {
		oldSecret = oldObj.(*corev1.Secret)
	}
	if secret == nil && oldSecret != nil {
		secret = oldSecret
	}

	// Skip system-generated namespaces
	if secret.Namespace == "kube-system" || secret.Namespace == "kube-public" || secret.Namespace == "kube-node-lease" {
		return
	}

	// Skip service account tokens and helm releases (system-generated)
	if secret.Type == corev1.SecretTypeServiceAccountToken ||
		string(secret.Type) == "helm.sh/release.v1" ||
		len(secret.OwnerReferences) > 0 {
		return
	}

	// For MODIFIED events, only track meaningful changes
	if eventType == watch.Modified && oldSecret != nil {
		hasChanges, changeDescription := w.detectSecretChanges(oldSecret, secret)
		if !hasChanges {
			return // Skip this event
		}

		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: secret.Namespace,
			Kind:      "Secret",
			Name:      secret.Name,
			Action:    string(eventType),
			Diff:      changeDescription,
		}

		// Extract metadata (keys only, never values)
		keys := make([]string, 0, len(secret.Data))
		for k := range secret.Data {
			keys = append(keys, k)
		}
		metadata := map[string]interface{}{
			"type": secret.Type,
			"keys": keys,
		}
		metadataJSON, _ := json.Marshal(metadata)
		event.Metadata = string(metadataJSON)

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving secret event: %v", err)
		} else {
			log.Printf("Saved %s event for secret %s/%s: %s", eventType, secret.Namespace, secret.Name, changeDescription)
		}
		return
	}

	// For ADDED/DELETED events, save them
	if eventType == watch.Added || eventType == watch.Deleted {
		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: secret.Namespace,
			Kind:      "Secret",
			Name:      secret.Name,
			Action:    string(eventType),
		}

		if eventType == watch.Added {
			keys := make([]string, 0, len(secret.Data))
			for k := range secret.Data {
				keys = append(keys, k)
			}
			event.Diff = fmt.Sprintf("Secret created (%s) with %d key(s)", secret.Type, len(keys))
		} else {
			event.Diff = "Secret deleted"
		}

		keys := make([]string, 0, len(secret.Data))
		for k := range secret.Data {
			keys = append(keys, k)
		}
		metadata := map[string]interface{}{
			"type": secret.Type,
			"keys": keys,
		}
		metadataJSON, _ := json.Marshal(metadata)
		event.Metadata = string(metadataJSON)

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving secret event: %v", err)
		} else {
			log.Printf("Saved %s event for secret %s/%s", eventType, secret.Namespace, secret.Name)
		}
	}
}

// detectSecretChanges checks for key additions, removals, or type changes
func (w *Watcher) detectSecretChanges(oldSecret, newSecret *corev1.Secret) (bool, string) {
	// Check for type change
	if oldSecret.Type != newSecret.Type {
		return true, fmt.Sprintf("Secret type changed: %s → %s", oldSecret.Type, newSecret.Type)
	}

	oldKeys := make(map[string]bool)
	for k := range oldSecret.Data {
		oldKeys[k] = true
	}

	newKeys := make(map[string]bool)
	for k := range newSecret.Data {
		newKeys[k] = true
	}

	// Check for added keys
	addedKeys := []string{}
	for k := range newSecret.Data {
		if !oldKeys[k] {
			addedKeys = append(addedKeys, k)
		}
	}

	// Check for removed keys
	removedKeys := []string{}
	for k := range oldSecret.Data {
		if !newKeys[k] {
			removedKeys = append(removedKeys, k)
		}
	}

	// Check for modified values (show which keys changed, not values for security)
	modifiedKeys := []string{}
	for k, newVal := range newSecret.Data {
		if oldVal, exists := oldSecret.Data[k]; exists {
			// Compare byte slices
			if string(oldVal) != string(newVal) {
				modifiedKeys = append(modifiedKeys, k)
			}
		}
	}

	if len(addedKeys) == 0 && len(removedKeys) == 0 && len(modifiedKeys) == 0 {
		return false, ""
	}

	// Build description
	if len(addedKeys) > 0 {
		return true, fmt.Sprintf("Keys added: %v", addedKeys)
	}
	if len(removedKeys) > 0 {
		return true, fmt.Sprintf("Keys removed: %v", removedKeys)
	}
	if len(modifiedKeys) > 0 {
		return true, fmt.Sprintf("Keys modified: %v\n\n(Secret values are not displayed for security)", modifiedKeys)
	}

	return false, ""
}

// saveAndNotify saves an event and sends notification
func (w *Watcher) saveAndNotify(event *storage.ChangeEvent) error {
	// Save to database
	if err := w.storage.SaveEvent(event); err != nil {
		return err
	}

	// Send Slack notification (non-blocking)
	if w.notifier.IsEnabled() {
		go func() {
			if err := w.notifier.NotifyChange(event); err != nil {
				log.Printf("Warning: Failed to send Slack notification: %v", err)
			}
		}()
	}

	return nil
}

// convertToMap converts a runtime object to a map for diffing
func convertToMap(obj runtime.Object) map[string]interface{} {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}
