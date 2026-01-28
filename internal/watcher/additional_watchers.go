package watcher

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"k8watch/internal/storage"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// watchServices watches service changes
func (w *Watcher) watchServices() {
	watchlist := cache.NewListWatchFromClient(
		w.clientset.CoreV1().RESTClient(),
		"services",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&corev1.Service{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				w.handleServiceEvent(watch.Added, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.handleServiceEvent(watch.Modified, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				w.handleServiceEvent(watch.Deleted, obj, nil)
			},
		},
	)

	controller.Run(w.stopCh)
}

func (w *Watcher) handleServiceEvent(eventType watch.EventType, oldObj, newObj interface{}) {
	var svc *corev1.Service
	var oldSvc *corev1.Service

	if newObj != nil {
		svc = newObj.(*corev1.Service)
	} else if oldObj != nil {
		svc = oldObj.(*corev1.Service)
	}

	if oldObj != nil {
		oldSvc = oldObj.(*corev1.Service)
	}

	if svc.Namespace == "kube-system" || svc.Namespace == "kube-public" || svc.Namespace == "kube-node-lease" {
		return
	}

	// For MODIFIED events, detect meaningful changes
	if eventType == watch.Modified && oldSvc != nil {
		hasChanges, changeDesc := w.detectServiceChanges(oldSvc, svc)
		if !hasChanges {
			return // Skip system-generated updates
		}

		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: svc.Namespace,
			Kind:      "Service",
			Name:      svc.Name,
			Action:    string(eventType),
			Diff:      changeDesc,
		}

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving service event: %v", err)
		} else {
			log.Printf("Saved %s event for service %s/%s", eventType, svc.Namespace, svc.Name)
		}
		return
	}

	// For ADDED/DELETED events
	event := &storage.ChangeEvent{
		Timestamp: time.Now(),
		Namespace: svc.Namespace,
		Kind:      "Service",
		Name:      svc.Name,
		Action:    string(eventType),
		Diff:      string(eventType),
	}

	if err := w.saveAndNotify(event); err != nil {
		log.Printf("Error saving service event: %v", err)
	} else {
		log.Printf("Saved %s event for service %s/%s", eventType, svc.Namespace, svc.Name)
	}
}

// detectServiceChanges checks for meaningful service changes
func (w *Watcher) detectServiceChanges(oldSvc, newSvc *corev1.Service) (bool, string) {
	changes := []string{}

	// Check service type changes
	if oldSvc.Spec.Type != newSvc.Spec.Type {
		changes = append(changes, fmt.Sprintf("Type: %s → %s", oldSvc.Spec.Type, newSvc.Spec.Type))
	}

	// Check selector changes
	if fmt.Sprintf("%v", oldSvc.Spec.Selector) != fmt.Sprintf("%v", newSvc.Spec.Selector) {
		changes = append(changes, "Selector changed")
	}

	// Check ports changes
	if len(oldSvc.Spec.Ports) != len(newSvc.Spec.Ports) {
		changes = append(changes, fmt.Sprintf("Ports count: %d → %d", len(oldSvc.Spec.Ports), len(newSvc.Spec.Ports)))
	} else {
		for i, newPort := range newSvc.Spec.Ports {
			if i < len(oldSvc.Spec.Ports) {
				oldPort := oldSvc.Spec.Ports[i]
				if oldPort.Port != newPort.Port || oldPort.TargetPort.IntVal != newPort.TargetPort.IntVal {
					changes = append(changes, fmt.Sprintf("Port %s: %d/%d → %d/%d", newPort.Name, oldPort.Port, oldPort.TargetPort.IntVal, newPort.Port, newPort.TargetPort.IntVal))
				}
			}
		}
	}

	// Check external IPs
	oldIPs := strings.Join(oldSvc.Spec.ExternalIPs, ",")
	newIPs := strings.Join(newSvc.Spec.ExternalIPs, ",")
	if oldIPs != newIPs {
		changes = append(changes, fmt.Sprintf("External IPs: %s → %s", oldIPs, newIPs))
	}

	if len(changes) == 0 {
		return false, ""
	}

	return true, "Service configuration changed:\n" + strings.Join(changes, "\n")
}

// watchIngresses watches ingress changes
func (w *Watcher) watchIngresses() {
	watchlist := cache.NewListWatchFromClient(
		w.clientset.NetworkingV1().RESTClient(),
		"ingresses",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&networkingv1.Ingress{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				w.handleIngressEvent(watch.Added, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.handleIngressEvent(watch.Modified, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				w.handleIngressEvent(watch.Deleted, obj, nil)
			},
		},
	)

	controller.Run(w.stopCh)
}

func (w *Watcher) handleIngressEvent(eventType watch.EventType, oldObj, newObj interface{}) {
	var ingress *networkingv1.Ingress
	var oldIngress *networkingv1.Ingress

	if newObj != nil {
		ingress = newObj.(*networkingv1.Ingress)
	} else if oldObj != nil {
		ingress = oldObj.(*networkingv1.Ingress)
	}

	if oldObj != nil {
		oldIngress = oldObj.(*networkingv1.Ingress)
	}

	if ingress.Namespace == "kube-system" || ingress.Namespace == "kube-public" || ingress.Namespace == "kube-node-lease" {
		return
	}

	// For MODIFIED events, detect meaningful changes
	if eventType == watch.Modified && oldIngress != nil {
		hasChanges, changeDesc := w.detectIngressChanges(oldIngress, ingress)
		if !hasChanges {
			return // Skip system-generated updates
		}

		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: ingress.Namespace,
			Kind:      "Ingress",
			Name:      ingress.Name,
			Action:    string(eventType),
			Diff:      changeDesc,
		}

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving ingress event: %v", err)
		} else {
			log.Printf("Saved %s event for ingress %s/%s", eventType, ingress.Namespace, ingress.Name)
		}
		return
	}

	// For ADDED/DELETED events
	event := &storage.ChangeEvent{
		Timestamp: time.Now(),
		Namespace: ingress.Namespace,
		Kind:      "Ingress",
		Name:      ingress.Name,
		Action:    string(eventType),
		Diff:      string(eventType),
	}

	if err := w.saveAndNotify(event); err != nil {
		log.Printf("Error saving ingress event: %v", err)
	} else {
		log.Printf("Saved %s event for ingress %s/%s", eventType, ingress.Namespace, ingress.Name)
	}
}

// detectIngressChanges checks for meaningful ingress changes
func (w *Watcher) detectIngressChanges(oldIng, newIng *networkingv1.Ingress) (bool, string) {
	changes := []string{}

	// Check annotation changes (important ones)
	importantAnnotations := []string{
		"nginx.ingress.kubernetes.io/rewrite-target",
		"cert-manager.io/cluster-issuer",
		"kubernetes.io/ingress.class",
		"konghq.com/",
	}

	for _, key := range importantAnnotations {
		oldVal, oldExists := oldIng.Annotations[key]
		newVal, newExists := newIng.Annotations[key]

		if oldExists != newExists || (oldExists && oldVal != newVal) {
			if newExists {
				changes = append(changes, fmt.Sprintf("Annotation %s: '%s' → '%s'", key, oldVal, newVal))
			} else {
				changes = append(changes, fmt.Sprintf("Annotation %s removed", key))
			}
		}
	}

	// Check for rules changes (hosts, paths, backends)
	if len(oldIng.Spec.Rules) != len(newIng.Spec.Rules) {
		changes = append(changes, fmt.Sprintf("Rules count: %d → %d", len(oldIng.Spec.Rules), len(newIng.Spec.Rules)))
	} else {
		// Check individual rules
		for i, newRule := range newIng.Spec.Rules {
			if i >= len(oldIng.Spec.Rules) {
				break
			}
			oldRule := oldIng.Spec.Rules[i]

			// Check host changes
			if oldRule.Host != newRule.Host {
				changes = append(changes, fmt.Sprintf("Host changed: %s → %s", oldRule.Host, newRule.Host))
			}

			// Check path changes
			if oldRule.HTTP != nil && newRule.HTTP != nil {
				if len(oldRule.HTTP.Paths) != len(newRule.HTTP.Paths) {
					changes = append(changes, fmt.Sprintf("Paths count for %s: %d → %d", newRule.Host, len(oldRule.HTTP.Paths), len(newRule.HTTP.Paths)))
				} else {
					for j, newPath := range newRule.HTTP.Paths {
						if j >= len(oldRule.HTTP.Paths) {
							break
						}
						oldPath := oldRule.HTTP.Paths[j]

						// Check backend service changes
						if oldPath.Backend.Service != nil && newPath.Backend.Service != nil {
							if oldPath.Backend.Service.Name != newPath.Backend.Service.Name {
								changes = append(changes, fmt.Sprintf("Backend service: %s → %s", oldPath.Backend.Service.Name, newPath.Backend.Service.Name))
							}
							if oldPath.Backend.Service.Port.Number != newPath.Backend.Service.Port.Number {
								changes = append(changes, fmt.Sprintf("Backend port: %d → %d", oldPath.Backend.Service.Port.Number, newPath.Backend.Service.Port.Number))
							}
						}
					}
				}
			}
		}
	}

	// Check TLS changes
	if len(oldIng.Spec.TLS) != len(newIng.Spec.TLS) {
		changes = append(changes, fmt.Sprintf("TLS config count: %d → %d", len(oldIng.Spec.TLS), len(newIng.Spec.TLS)))
	}

	if len(changes) == 0 {
		return false, ""
	}

	return true, "Ingress configuration changed:\n" + strings.Join(changes, "\n")
}

// watchStatefulSets watches statefulset changes
func (w *Watcher) watchStatefulSets() {
	watchlist := cache.NewListWatchFromClient(
		w.clientset.AppsV1().RESTClient(),
		"statefulsets",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&appsv1.StatefulSet{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				w.handleStatefulSetEvent(watch.Added, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.handleStatefulSetEvent(watch.Modified, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				w.handleStatefulSetEvent(watch.Deleted, obj, nil)
			},
		},
	)

	controller.Run(w.stopCh)
}

func (w *Watcher) handleStatefulSetEvent(eventType watch.EventType, oldObj, newObj interface{}) {
	var ss *appsv1.StatefulSet
	var oldSS *appsv1.StatefulSet

	if newObj != nil {
		ss = newObj.(*appsv1.StatefulSet)
	} else if oldObj != nil {
		ss = oldObj.(*appsv1.StatefulSet)
	}

	if oldObj != nil {
		oldSS = oldObj.(*appsv1.StatefulSet)
	}

	if ss.Namespace == "kube-system" || ss.Namespace == "kube-public" || ss.Namespace == "kube-node-lease" {
		return
	}

	// For updates, check if there are meaningful changes
	if eventType == watch.Modified && oldSS != nil {
		hasChanges, diff := w.detectStatefulSetChanges(oldSS, ss)
		if !hasChanges {
			return // Ignore system-generated updates
		}

		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: ss.Namespace,
			Kind:      "StatefulSet",
			Name:      ss.Name,
			Action:    string(eventType),
			Diff:      diff,
		}

		metadata := map[string]interface{}{
			"replicas": ss.Spec.Replicas,
		}
		metadataJSON, _ := json.Marshal(metadata)
		event.Metadata = string(metadataJSON)

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving statefulset event: %v", err)
		} else {
			log.Printf("Saved %s event for statefulset %s/%s", eventType, ss.Namespace, ss.Name)
		}
		return
	}

	event := &storage.ChangeEvent{
		Timestamp: time.Now(),
		Namespace: ss.Namespace,
		Kind:      "StatefulSet",
		Name:      ss.Name,
		Action:    string(eventType),
		Diff:      string(eventType),
	}

	if err := w.saveAndNotify(event); err != nil {
		log.Printf("Error saving statefulset event: %v", err)
	} else {
		log.Printf("Saved %s event for statefulset %s/%s", eventType, ss.Namespace, ss.Name)
	}
}

// detectStatefulSetChanges checks for meaningful statefulset changes
func (w *Watcher) detectStatefulSetChanges(oldSS, newSS *appsv1.StatefulSet) (bool, string) {
	changes := []string{}

	// Check replica count changes
	if oldSS.Spec.Replicas != nil && newSS.Spec.Replicas != nil && *oldSS.Spec.Replicas != *newSS.Spec.Replicas {
		changes = append(changes, fmt.Sprintf("Replicas: %d → %d", *oldSS.Spec.Replicas, *newSS.Spec.Replicas))
	}

	// Check image changes
	if len(oldSS.Spec.Template.Spec.Containers) > 0 && len(newSS.Spec.Template.Spec.Containers) > 0 {
		for i, newContainer := range newSS.Spec.Template.Spec.Containers {
			if i < len(oldSS.Spec.Template.Spec.Containers) {
				oldContainer := oldSS.Spec.Template.Spec.Containers[i]
				if oldContainer.Image != newContainer.Image {
					changes = append(changes, fmt.Sprintf("Container %s image: %s → %s", newContainer.Name, oldContainer.Image, newContainer.Image))
				}
			}
		}
	}

	// Check service name changes
	if oldSS.Spec.ServiceName != newSS.Spec.ServiceName {
		changes = append(changes, fmt.Sprintf("Service name: %s → %s", oldSS.Spec.ServiceName, newSS.Spec.ServiceName))
	}

	// Check volume claim template changes
	if len(oldSS.Spec.VolumeClaimTemplates) != len(newSS.Spec.VolumeClaimTemplates) {
		changes = append(changes, fmt.Sprintf("Volume claim templates: %d → %d", len(oldSS.Spec.VolumeClaimTemplates), len(newSS.Spec.VolumeClaimTemplates)))
	}

	// Check update strategy
	if oldSS.Spec.UpdateStrategy.Type != newSS.Spec.UpdateStrategy.Type {
		changes = append(changes, fmt.Sprintf("Update strategy: %s → %s", oldSS.Spec.UpdateStrategy.Type, newSS.Spec.UpdateStrategy.Type))
	}

	if len(changes) == 0 {
		return false, ""
	}

	return true, "StatefulSet configuration changed:\n" + strings.Join(changes, "\n")
}

// watchDaemonSets watches daemonset changes
func (w *Watcher) watchDaemonSets() {
	watchlist := cache.NewListWatchFromClient(
		w.clientset.AppsV1().RESTClient(),
		"daemonsets",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&appsv1.DaemonSet{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				w.handleDaemonSetEvent(watch.Added, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.handleDaemonSetEvent(watch.Modified, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				w.handleDaemonSetEvent(watch.Deleted, obj, nil)
			},
		},
	)

	controller.Run(w.stopCh)
}

func (w *Watcher) handleDaemonSetEvent(eventType watch.EventType, oldObj, newObj interface{}) {
	var ds *appsv1.DaemonSet
	var oldDS *appsv1.DaemonSet

	if newObj != nil {
		ds = newObj.(*appsv1.DaemonSet)
	} else if oldObj != nil {
		ds = oldObj.(*appsv1.DaemonSet)
	}

	if oldObj != nil {
		oldDS = oldObj.(*appsv1.DaemonSet)
	}

	if ds.Namespace == "kube-system" || ds.Namespace == "kube-public" || ds.Namespace == "kube-node-lease" {
		return
	}

	// For updates, check if there are meaningful changes
	if eventType == watch.Modified && oldDS != nil {
		hasChanges, diff := w.detectDaemonSetChanges(oldDS, ds)
		if !hasChanges {
			return // Ignore system-generated updates
		}

		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: ds.Namespace,
			Kind:      "DaemonSet",
			Name:      ds.Name,
			Action:    string(eventType),
			Diff:      diff,
		}

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving daemonset event: %v", err)
		} else {
			log.Printf("Saved %s event for daemonset %s/%s", eventType, ds.Namespace, ds.Name)
		}
		return
	}

	event := &storage.ChangeEvent{
		Timestamp: time.Now(),
		Namespace: ds.Namespace,
		Kind:      "DaemonSet",
		Name:      ds.Name,
		Action:    string(eventType),
		Diff:      string(eventType),
	}

	if err := w.saveAndNotify(event); err != nil {
		log.Printf("Error saving daemonset event: %v", err)
	} else {
		log.Printf("Saved %s event for daemonset %s/%s", eventType, ds.Namespace, ds.Name)
	}
}

// detectDaemonSetChanges checks for meaningful daemonset changes
func (w *Watcher) detectDaemonSetChanges(oldDS, newDS *appsv1.DaemonSet) (bool, string) {
	changes := []string{}

	// Check image changes
	if len(oldDS.Spec.Template.Spec.Containers) > 0 && len(newDS.Spec.Template.Spec.Containers) > 0 {
		for i, newContainer := range newDS.Spec.Template.Spec.Containers {
			if i < len(oldDS.Spec.Template.Spec.Containers) {
				oldContainer := oldDS.Spec.Template.Spec.Containers[i]
				if oldContainer.Image != newContainer.Image {
					changes = append(changes, fmt.Sprintf("Container %s image: %s → %s", newContainer.Name, oldContainer.Image, newContainer.Image))
				}
			}
		}
	}

	// Check update strategy
	if oldDS.Spec.UpdateStrategy.Type != newDS.Spec.UpdateStrategy.Type {
		changes = append(changes, fmt.Sprintf("Update strategy: %s → %s", oldDS.Spec.UpdateStrategy.Type, newDS.Spec.UpdateStrategy.Type))
	}

	// Check node selector changes
	if fmt.Sprintf("%v", oldDS.Spec.Template.Spec.NodeSelector) != fmt.Sprintf("%v", newDS.Spec.Template.Spec.NodeSelector) {
		changes = append(changes, "Node selector changed")
	}

	if len(changes) == 0 {
		return false, ""
	}

	return true, "DaemonSet configuration changed:\n" + strings.Join(changes, "\n")
}

// watchCronJobs watches cronjob changes
func (w *Watcher) watchCronJobs() {
	watchlist := cache.NewListWatchFromClient(
		w.clientset.BatchV1().RESTClient(),
		"cronjobs",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&batchv1.CronJob{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				w.handleCronJobEvent(watch.Added, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.handleCronJobEvent(watch.Modified, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				w.handleCronJobEvent(watch.Deleted, obj, nil)
			},
		},
	)

	controller.Run(w.stopCh)
}

func (w *Watcher) handleCronJobEvent(eventType watch.EventType, oldObj, newObj interface{}) {
	var cronjob *batchv1.CronJob
	var oldCronJob *batchv1.CronJob

	if newObj != nil {
		cronjob = newObj.(*batchv1.CronJob)
	} else if oldObj != nil {
		cronjob = oldObj.(*batchv1.CronJob)
	}

	if oldObj != nil {
		oldCronJob = oldObj.(*batchv1.CronJob)
	}

	if cronjob.Namespace == "kube-system" || cronjob.Namespace == "kube-public" || cronjob.Namespace == "kube-node-lease" {
		return
	}

	// For updates, check if there are meaningful changes
	if eventType == watch.Modified && oldCronJob != nil {
		hasChanges, diff := w.detectCronJobChanges(oldCronJob, cronjob)
		if !hasChanges {
			return // Ignore system-generated updates
		}

		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: cronjob.Namespace,
			Kind:      "CronJob",
			Name:      cronjob.Name,
			Action:    string(eventType),
			Diff:      diff,
		}

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving cronjob event: %v", err)
		} else {
			log.Printf("Saved %s event for cronjob %s/%s", eventType, cronjob.Namespace, cronjob.Name)
		}
		return
	}

	event := &storage.ChangeEvent{
		Timestamp: time.Now(),
		Namespace: cronjob.Namespace,
		Kind:      "CronJob",
		Name:      cronjob.Name,
		Action:    string(eventType),
		Diff:      string(eventType),
	}

	if err := w.saveAndNotify(event); err != nil {
		log.Printf("Error saving cronjob event: %v", err)
	} else {
		log.Printf("Saved %s event for cronjob %s/%s", eventType, cronjob.Namespace, cronjob.Name)
	}
}

// detectCronJobChanges checks for meaningful cronjob changes
func (w *Watcher) detectCronJobChanges(oldCJ, newCJ *batchv1.CronJob) (bool, string) {
	changes := []string{}

	// Check schedule changes
	if oldCJ.Spec.Schedule != newCJ.Spec.Schedule {
		changes = append(changes, fmt.Sprintf("Schedule: %s → %s", oldCJ.Spec.Schedule, newCJ.Spec.Schedule))
	}

	// Check suspend status
	oldSuspend := oldCJ.Spec.Suspend != nil && *oldCJ.Spec.Suspend
	newSuspend := newCJ.Spec.Suspend != nil && *newCJ.Spec.Suspend
	if oldSuspend != newSuspend {
		changes = append(changes, fmt.Sprintf("Suspend: %v → %v", oldSuspend, newSuspend))
	}

	// Check image changes in job template
	if len(oldCJ.Spec.JobTemplate.Spec.Template.Spec.Containers) > 0 && len(newCJ.Spec.JobTemplate.Spec.Template.Spec.Containers) > 0 {
		for i, newContainer := range newCJ.Spec.JobTemplate.Spec.Template.Spec.Containers {
			if i < len(oldCJ.Spec.JobTemplate.Spec.Template.Spec.Containers) {
				oldContainer := oldCJ.Spec.JobTemplate.Spec.Template.Spec.Containers[i]
				if oldContainer.Image != newContainer.Image {
					changes = append(changes, fmt.Sprintf("Container %s image: %s → %s", newContainer.Name, oldContainer.Image, newContainer.Image))
				}
			}
		}
	}

	// Check concurrency policy
	if oldCJ.Spec.ConcurrencyPolicy != newCJ.Spec.ConcurrencyPolicy {
		changes = append(changes, fmt.Sprintf("Concurrency policy: %s → %s", oldCJ.Spec.ConcurrencyPolicy, newCJ.Spec.ConcurrencyPolicy))
	}

	if len(changes) == 0 {
		return false, ""
	}

	return true, "CronJob configuration changed:\n" + strings.Join(changes, "\n")
}

// watchJobs watches job changes
func (w *Watcher) watchJobs() {
	watchlist := cache.NewListWatchFromClient(
		w.clientset.BatchV1().RESTClient(),
		"jobs",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&batchv1.Job{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				w.handleJobEvent(watch.Added, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.handleJobEvent(watch.Modified, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				w.handleJobEvent(watch.Deleted, obj, nil)
			},
		},
	)

	controller.Run(w.stopCh)
}

func (w *Watcher) handleJobEvent(eventType watch.EventType, oldObj, newObj interface{}) {
	var job *batchv1.Job
	var oldJob *batchv1.Job

	if newObj != nil {
		job = newObj.(*batchv1.Job)
	} else if oldObj != nil {
		job = oldObj.(*batchv1.Job)
	}

	if oldObj != nil {
		oldJob = oldObj.(*batchv1.Job)
	}

	if job.Namespace == "kube-system" || job.Namespace == "kube-public" || job.Namespace == "kube-node-lease" {
		return
	}

	// For updates, check if there are meaningful changes
	if eventType == watch.Modified && oldJob != nil {
		// Skip status-only updates (completion, progress)
		if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
			return
		}

		hasChanges, diff := w.detectJobChanges(oldJob, job)
		if !hasChanges {
			return // Ignore system-generated updates
		}

		event := &storage.ChangeEvent{
			Timestamp: time.Now(),
			Namespace: job.Namespace,
			Kind:      "Job",
			Name:      job.Name,
			Action:    string(eventType),
			Diff:      diff,
		}

		if err := w.saveAndNotify(event); err != nil {
			log.Printf("Error saving job event: %v", err)
		} else {
			log.Printf("Saved %s event for job %s/%s", eventType, job.Namespace, job.Name)
		}
		return
	}

	event := &storage.ChangeEvent{
		Timestamp: time.Now(),
		Namespace: job.Namespace,
		Kind:      "Job",
		Name:      job.Name,
		Action:    string(eventType),
		Diff:      string(eventType),
	}

	if err := w.saveAndNotify(event); err != nil {
		log.Printf("Error saving job event: %v", err)
	} else {
		log.Printf("Saved %s event for job %s/%s", eventType, job.Namespace, job.Name)
	}
}

// detectJobChanges checks for meaningful job changes
func (w *Watcher) detectJobChanges(oldJob, newJob *batchv1.Job) (bool, string) {
	changes := []string{}

	// Check parallelism changes
	if oldJob.Spec.Parallelism != nil && newJob.Spec.Parallelism != nil && *oldJob.Spec.Parallelism != *newJob.Spec.Parallelism {
		changes = append(changes, fmt.Sprintf("Parallelism: %d → %d", *oldJob.Spec.Parallelism, *newJob.Spec.Parallelism))
	}

	// Check completions changes
	if oldJob.Spec.Completions != nil && newJob.Spec.Completions != nil && *oldJob.Spec.Completions != *newJob.Spec.Completions {
		changes = append(changes, fmt.Sprintf("Completions: %d → %d", *oldJob.Spec.Completions, *newJob.Spec.Completions))
	}

	// Check image changes
	if len(oldJob.Spec.Template.Spec.Containers) > 0 && len(newJob.Spec.Template.Spec.Containers) > 0 {
		for i, newContainer := range newJob.Spec.Template.Spec.Containers {
			if i < len(oldJob.Spec.Template.Spec.Containers) {
				oldContainer := oldJob.Spec.Template.Spec.Containers[i]
				if oldContainer.Image != newContainer.Image {
					changes = append(changes, fmt.Sprintf("Container %s image: %s → %s", newContainer.Name, oldContainer.Image, newContainer.Image))
				}
			}
		}
	}

	// Check backoff limit changes
	if oldJob.Spec.BackoffLimit != nil && newJob.Spec.BackoffLimit != nil && *oldJob.Spec.BackoffLimit != *newJob.Spec.BackoffLimit {
		changes = append(changes, fmt.Sprintf("Backoff limit: %d → %d", *oldJob.Spec.BackoffLimit, *newJob.Spec.BackoffLimit))
	}

	if len(changes) == 0 {
		return false, ""
	}

	return true, "Job configuration changed:\n" + strings.Join(changes, "\n")
}
