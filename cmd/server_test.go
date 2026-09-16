package cmd

import (
	"testing"

	"github.com/ayates83/kubectl-neat/pkg/testutil"
)

func TestNeatServerPopulated(t *testing.T) {
	cases := []struct {
		title string
		data  string
		want  string
	}{
		{
			title: "pod: admission-injected priority, account, tolerations, token volume",
			data: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","labels":{"app":"a","pod-template-hash":"abc123"}},
				"spec":{"priority":0,"preemptionPolicy":"PreemptLowerPriority","serviceAccountName":"default","serviceAccount":"default",
				"containers":[{"name":"c","image":"i","volumeMounts":[{"name":"data","mountPath":"/d"},{"name":"kube-api-access-x7k2p","mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}]}],
				"initContainers":[{"name":"init","image":"i","volumeMounts":[{"name":"kube-api-access-x7k2p","mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}]}],
				"volumes":[{"name":"data","emptyDir":{}},{"name":"kube-api-access-x7k2p","projected":{}}],
				"tolerations":[
					{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},
					{"key":"dedicated","operator":"Equal","value":"gpu","effect":"NoSchedule"},
					{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}]}}`,
			want: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","labels":{"app":"a"}},
				"spec":{
				"containers":[{"name":"c","image":"i","volumeMounts":[{"name":"data","mountPath":"/d"}]}],
				"initContainers":[{"name":"init","image":"i","volumeMounts":[]}],
				"volumes":[{"name":"data","emptyDir":{}}],
				"tolerations":[{"key":"dedicated","operator":"Equal","value":"gpu","effect":"NoSchedule"}]}}`,
		},
		{
			title: "pod: a toleration someone tuned, and a non-default account, are kept",
			data: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"},"spec":{"serviceAccountName":"builder",
				"imagePullSecrets":[{"name":"builder-dockercfg-q8w2z"},{"name":"registry-creds"}],
				"tolerations":[{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}]}}`,
			want: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"},"spec":{"serviceAccountName":"builder",
				"imagePullSecrets":[{"name":"registry-creds"}],
				"tolerations":[{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}]}}`,
		},
		{
			title: "pod: OpenShift SCC namespace allocations go",
			data: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","annotations":{"openshift.io/scc":"restricted-v2"}},
				"spec":{"securityContext":{"fsGroup":1000810000,"seLinuxOptions":{"level":"s0:c28,c27"},"seccompProfile":{"type":"RuntimeDefault"}},
				"containers":[{"name":"c","image":"i","securityContext":{"runAsNonRoot":true,"runAsUser":1000810000}}],
				"initContainers":[{"name":"init","image":"i","securityContext":{"runAsUser":1000810000}}]}}`,
			want: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","annotations":{}},
				"spec":{"securityContext":{"seccompProfile":{"type":"RuntimeDefault"}},
				"containers":[{"name":"c","image":"i","securityContext":{"runAsNonRoot":true}}],
				"initContainers":[{"name":"init","image":"i","securityContext":{}}]}}`,
		},
		{
			title: "pod: authored IDs and SELinux options stay, and nothing is touched without an SCC",
			data: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","annotations":{"openshift.io/scc":"anyuid"}},
				"spec":{"securityContext":{"fsGroup":2000,"seLinuxOptions":{"type":"spc_t","level":"s0:c1,c2"}},
				"containers":[{"name":"c","image":"i","securityContext":{"runAsUser":1001}},{"name":"d","image":"i","securityContext":{"runAsUser":1000810001}}]}}`,
			want: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","annotations":{}},
				"spec":{"securityContext":{"fsGroup":2000,"seLinuxOptions":{"type":"spc_t","level":"s0:c1,c2"}},
				"containers":[{"name":"c","image":"i","securityContext":{"runAsUser":1001}},{"name":"d","image":"i","securityContext":{"runAsUser":1000810001}}]}}`,
		},
		{
			title: "pod: without openshift.io/scc an allocator-shaped ID is left alone",
			data:  `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"},"spec":{"securityContext":{"runAsUser":1000810000,"seLinuxOptions":{"level":"s0:c28,c27"}}}}`,
			want:  `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"},"spec":{"securityContext":{"runAsUser":1000810000,"seLinuxOptions":{"level":"s0:c28,c27"}}}}`,
		},
		{
			title: "pod: job labels are only stripped when the controller-uid proves they are generated",
			data:  `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","labels":{"job-name":"mine"}},"spec":{}}`,
			want:  `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","labels":{"job-name":"mine"}},"spec":{}}`,
		},
		{
			title: "replicaset: pod-template-hash is part of the selector and stays",
			data:  `{"apiVersion":"apps/v1","kind":"ReplicaSet","metadata":{"name":"r","labels":{"pod-template-hash":"abc"}},"spec":{}}`,
			want:  `{"apiVersion":"apps/v1","kind":"ReplicaSet","metadata":{"name":"r","labels":{"pod-template-hash":"abc"}},"spec":{}}`,
		},
		{
			title: "job: generated selector and labels go, so the Job can be re-applied",
			data: `{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"j","labels":{"batch.kubernetes.io/controller-uid":"u","batch.kubernetes.io/job-name":"j","controller-uid":"u","job-name":"j"}},
				"spec":{"selector":{"matchLabels":{"batch.kubernetes.io/controller-uid":"u"}},
				"template":{"metadata":{"creationTimestamp":null,"labels":{"app":"x","batch.kubernetes.io/controller-uid":"u","batch.kubernetes.io/job-name":"j","controller-uid":"u","job-name":"j"}},"spec":{}}}}`,
			want: `{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"j","labels":{}},
				"spec":{"template":{"metadata":{"labels":{"app":"x"}},"spec":{}}}}`,
		},
		{
			title: "job: a manual selector is authored and stays",
			data:  `{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"j"},"spec":{"manualSelector":true,"selector":{"matchLabels":{"job-name":"j"}},"template":{"metadata":{"labels":{"job-name":"j"}}}}}`,
			want:  `{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"j"},"spec":{"manualSelector":true,"selector":{"matchLabels":{"job-name":"j"}},"template":{"metadata":{"labels":{"job-name":"j"}}}}}`,
		},
		{
			title: "service: allocated ClusterIP and single-stack IPv4 go",
			data:  `{"apiVersion":"v1","kind":"Service","metadata":{"name":"s"},"spec":{"clusterIP":"172.30.1.2","clusterIPs":["172.30.1.2"],"ipFamilies":["IPv4"],"ipFamilyPolicy":"SingleStack","ports":[{"port":80}]}}`,
			want:  `{"apiVersion":"v1","kind":"Service","metadata":{"name":"s"},"spec":{"ports":[{"port":80}]}}`,
		},
		{
			title: "service: headless and requested dual-stack are authored and stay",
			data:  `{"apiVersion":"v1","kind":"Service","metadata":{"name":"s"},"spec":{"clusterIP":"None","clusterIPs":["None"],"ipFamilies":["IPv4","IPv6"],"ipFamilyPolicy":"PreferDualStack"}}`,
			want:  `{"apiVersion":"v1","kind":"Service","metadata":{"name":"s"},"spec":{"clusterIP":"None","clusterIPs":["None"],"ipFamilies":["IPv4","IPv6"],"ipFamilyPolicy":"PreferDualStack"}}`,
		},
		{
			title: "pvc: a controller-chosen volume and binder annotations go",
			data: `{"apiVersion":"v1","kind":"PersistentVolumeClaim","metadata":{"name":"c","annotations":{"pv.kubernetes.io/bind-completed":"yes","pv.kubernetes.io/bound-by-controller":"yes","volume.kubernetes.io/storage-provisioner":"csi.example.com","volume.kubernetes.io/selected-node":"worker-0"}},
				"spec":{"volumeName":"pvc-0000","resources":{"requests":{"storage":"1Gi"}}}}`,
			want: `{"apiVersion":"v1","kind":"PersistentVolumeClaim","metadata":{"name":"c","annotations":{}},"spec":{"resources":{"requests":{"storage":"1Gi"}}}}`,
		},
		{
			title: "pvc: a statically requested volume stays",
			data:  `{"apiVersion":"v1","kind":"PersistentVolumeClaim","metadata":{"name":"c","annotations":{"pv.kubernetes.io/bind-completed":"yes"}},"spec":{"volumeName":"nfs-share"}}`,
			want:  `{"apiVersion":"v1","kind":"PersistentVolumeClaim","metadata":{"name":"c","annotations":{}},"spec":{"volumeName":"nfs-share"}}`,
		},
		{
			title: "pv: provisioned-by is load-bearing and stays",
			data:  `{"apiVersion":"v1","kind":"PersistentVolume","metadata":{"name":"v","annotations":{"pv.kubernetes.io/provisioned-by":"csi.example.com","pv.kubernetes.io/bound-by-controller":"yes"}}}`,
			want:  `{"apiVersion":"v1","kind":"PersistentVolume","metadata":{"name":"v","annotations":{"pv.kubernetes.io/provisioned-by":"csi.example.com"}}}`,
		},
		{
			title: "namespace: name label, finalizer, and per-cluster OpenShift allocations go",
			data: `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"n","labels":{"kubernetes.io/metadata.name":"n","team":"t"},
				"annotations":{"openshift.io/sa.scc.uid-range":"1000660000/10000","openshift.io/sa.scc.mcs":"s0:c26,c5","openshift.io/sa.scc.supplemental-groups":"1000660000/10000","openshift.io/requester":"someone"}},
				"spec":{"finalizers":["kubernetes"]}}`,
			want: `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"n","labels":{"team":"t"},"annotations":{}},"spec":{}}`,
		},
		{
			title: "route: a generated host and API defaults go",
			data: `{"apiVersion":"route.openshift.io/v1","kind":"Route","metadata":{"name":"r","annotations":{"openshift.io/host.generated":"true"}},
				"spec":{"host":"r-ns.apps.example.com","to":{"kind":"Service","name":"s","weight":100},"wildcardPolicy":"None"}}`,
			want: `{"apiVersion":"route.openshift.io/v1","kind":"Route","metadata":{"name":"r","annotations":{}},"spec":{"to":{"name":"s"}}}`,
		},
		{
			title: "route: an explicit host and a weighted backend stay",
			data:  `{"apiVersion":"route.openshift.io/v1","kind":"Route","metadata":{"name":"r"},"spec":{"host":"shop.example.com","to":{"kind":"Service","name":"s","weight":80}}}`,
			want:  `{"apiVersion":"route.openshift.io/v1","kind":"Route","metadata":{"name":"r"},"spec":{"host":"shop.example.com","to":{"name":"s","weight":80}}}`,
		},
		{
			title: "deployment: rollout history and GitOps tracking go from object and template, authored keys stay",
			data: `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"d",
				"annotations":{"deployment.kubernetes.io/revision":"4","argocd.argoproj.io/tracking-id":"app:apps/Deployment:ns/d","owner":"team-a"},
				"labels":{"kustomize.toolkit.fluxcd.io/name":"apps","kustomize.toolkit.fluxcd.io/namespace":"flux-system","app":"d"}},
				"spec":{"template":{"metadata":{"creationTimestamp":null,"annotations":{"kubectl.kubernetes.io/restartedAt":"2026-01-01T00:00:00Z","prometheus.io/scrape":"true"}}}}}`,
			want: `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"d","annotations":{"owner":"team-a"},"labels":{"app":"d"}},
				"spec":{"template":{"metadata":{"annotations":{"prometheus.io/scrape":"true"}}}}}`,
		},
		{
			title: "statefulset: volumeClaimTemplates metadata is cleaned too",
			data:  `{"apiVersion":"apps/v1","kind":"StatefulSet","metadata":{"name":"s"},"spec":{"volumeClaimTemplates":[{"metadata":{"name":"data","creationTimestamp":null},"spec":{}}]}}`,
			want:  `{"apiVersion":"apps/v1","kind":"StatefulSet","metadata":{"name":"s"},"spec":{"volumeClaimTemplates":[{"metadata":{"name":"data"},"spec":{}}]}}`,
		},
	}
	for _, c := range cases {
		have, err := neatServerPopulated(c.data)
		if err != nil {
			t.Errorf("error in neatServerPopulated for case '%s': %v", c.title, err)
			continue
		}
		equal, err := testutil.JSONEqual(have, c.want)
		if err != nil {
			t.Errorf("error in JSONEqual for case '%s': %v", c.title, err)
			continue
		}
		if !equal {
			t.Errorf("test case '%s' failed.\nwant: %s\nhave: %s", c.title, c.want, have)
		}
	}
}

func TestUserStripLists(t *testing.T) {
	defer func() { userAnnotations, userLabels = nil, nil }()
	userAnnotations = []string{"example.com/*", "ticket"}
	userLabels = []string{"app", "team"}

	data := `{"apiVersion":"apps/v1","kind":"Deployment",
		"metadata":{"name":"d","annotations":{"example.com/owner":"x","example.com/":"y","ticket":"1","tickets":"2"},"labels":{"app":"d","team":"t"}},
		"spec":{"selector":{"matchLabels":{"app":"d"}},"template":{"metadata":{"labels":{"app":"d","team":"t"}}}}}`
	want := `{"apiVersion":"apps/v1","kind":"Deployment",
		"metadata":{"name":"d","annotations":{"tickets":"2"},"labels":{}},
		"spec":{"selector":{"matchLabels":{"app":"d"}},"template":{"metadata":{"labels":{"app":"d"}}}}}`
	have, err := neatServerPopulated(data)
	if err != nil {
		t.Fatal(err)
	}
	if equal, _ := testutil.JSONEqual(have, want); !equal {
		t.Errorf("a selector label must survive on the template, prefix and exact matches must not overreach.\nwant: %s\nhave: %s", want, have)
	}
}
