package main

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

func resourceKinds() []string {
	return []string{"workloads.kueue.x-k8s.io", "localqueues.kueue.x-k8s.io", "clusterqueues.kueue.x-k8s.io", "resourceflavors.kueue.x-k8s.io", "pods"}
}
func clusterFind(o []Obj, kind, name string) Obj {
	for _, x := range o {
		if s(x, "kind") == kind && s(x, "metadata", "name") == name {
			return x
		}
	}
	return nil
}
func diagnose(r *Report, o, b []Obj, n, name, src, dst string, port int) {
	w := find(o, "Workload", n, name)
	if w == nil {
		missing(r, "Workload", n, name)
		return
	}
	admitted := false
	conditions := arr(w, "status", "conditions")
	if len(conditions) == 0 {
		r.add("admission-unknown", "warning", "unknown", "No Workload conditions available", w, "status.conditions")
	}
	for _, c := range conditions {
		r.add("workload-condition", "info", "confirmed", s(c, "type")+"="+s(c, "status")+" reason="+s(c, "reason"), w, "status.conditions")
		if s(c, "type") == "Admitted" && s(c, "status") == "True" {
			admitted = true
		}
	}
	totals := map[string]resource.Quantity{}
	for _, ps := range arr(w, "spec", "podSets") {
		count := num(ps, "count")
		for _, c := range arr(ps, "template", "spec", "containers") {
			requests, _ := get(c, "resources", "requests").(map[string]interface{})
			limits, _ := get(c, "resources", "limits").(map[string]interface{})
			all := map[string]interface{}{}
			for k, v := range limits {
				if strings.Contains(k, "/") {
					all[k] = v
				}
			}
			for k, v := range requests {
				all[k] = v
			}
			for k, v := range all {
				if !strings.Contains(strings.ToLower(k), "gpu") && !strings.Contains(strings.ToLower(k), "mig-") {
					continue
				}
				q, e := resource.ParseQuantity(fmt.Sprint(v))
				if e != nil {
					r.add("invalid-gpu-quantity", "warning", "unknown", "Could not parse GPU resource quantity", w, "spec.podSets")
					continue
				}
				q.Mul(int64(count))
				total := totals[k]
				total.Add(q)
				totals[k] = total
			}
		}
		if len(arr(ps, "template", "spec", "initContainers")) > 0 || get(ps, "template", "spec", "overhead") != nil {
			r.Limitations = append(r.Limitations, "Init container and Pod overhead scheduling requests are not included in regular-container GPU totals.")
		}
	}
	for k, q := range totals {
		r.add("gpu-request", "info", "confirmed", k+" regular-container total="+q.String(), w, "spec.podSets")
	}
	queueName := s(w, "spec", "queueName")
	lq := find(o, "LocalQueue", n, queueName)
	if lq == nil {
		missing(r, "LocalQueue", n, queueName)
		return
	}
	cqName := s(lq, "spec", "clusterQueue")
	cq := clusterFind(o, "ClusterQueue", cqName)
	if cq == nil {
		missing(r, "ClusterQueue", "", cqName)
		return
	}
	r.add("queue-linked", "info", "confirmed", "LocalQueue -> ClusterQueue "+cqName, lq, "spec.clusterQueue")
	for _, group := range arr(cq, "spec", "resourceGroups") {
		for _, flavor := range arr(group, "flavors") {
			fname := s(flavor, "name")
			f := clusterFind(o, "ResourceFlavor", fname)
			if f == nil {
				missing(r, "ResourceFlavor", "", fname)
			} else {
				r.add("flavor-linked", "info", "confirmed", "ResourceFlavor "+fname+" linked; node feasibility is not simulated", f, "spec.nodeLabels / spec.nodeTaints")
			}
			for _, res := range arr(flavor, "resources") {
				key := s(res, "name")
				req, ok := totals[key]
				if !ok {
					continue
				}
				quota, e := resource.ParseQuantity(s(res, "nominalQuota"))
				if e != nil {
					r.add("quota-unknown", "warning", "unknown", "Quota quantity unavailable", cq, "spec.resourceGroups")
					continue
				}
				r.add("quota-observed", "info", "confirmed", key+" flavor="+fname+" nominalQuota="+quota.String(), cq, "spec.resourceGroups")
				if req.Cmp(quota) > 0 {
					r.add("request-over-nominal-quota", "warning", "predicted", "Regular-container request exceeds this flavor's nominal quota; borrowing and flavor assignment may change admission", cq, "spec.resourceGroups")
				}
			}
		}
	}
	if admitted {
		r.add("admission-complete", "info", "confirmed", "Kueue admission completed; Pod scheduling is a separate stage", w, "status.conditions")
		for _, p := range o {
			if s(p, "kind") != "Pod" || ns(p) != n || s(p, "status", "phase") != "Pending" {
				continue
			}
			linked := s(p, "metadata", "labels", "kueue.x-k8s.io/workload-name") == name
			if linked {
				r.add("admitted-pod-pending", "warning", "confirmed", "Associated Pod remains Pending after admission", p, "status.phase")
			}
		}
	}
	r.Limitations = append(r.Limitations, "No wait-time prediction or full scheduler simulation. Cohort borrowing, current reservation accounting, topology, affinity, taints and multi-flavor allocation are not fully evaluated. Workload condition messages are omitted to avoid disclosing arbitrary content.")
}
