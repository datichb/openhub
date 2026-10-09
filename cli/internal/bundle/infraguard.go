package bundle

import "github.com/datichb/openhub/cli/internal/sessionspec"

// Infrastructure guard (v5 critical N4, DC4). The agents that may run
// infrastructure tools (their own rules do not refuse the tool) get, after
// their own rules:
//   - the read-only commands allowed (anchored on the tool: a global option
//     before the sub-command falls back to the agent's own rule);
//   - the reads of secrets and the tunnels asked;
//   - every mutation asked — and refused by the session rules until a
//     checkpoint `unlocks: [apply]` is passed (always refused in a workflow
//     that declares none, ApplyLockedByDefault).
//
// Credential extraction is refused to every agent by the global guard
// (credentialDeny). `terraform plan` and `pulumi preview` run code of the
// repository (providers, programs): the real protection is read-only
// credentials (ReadOnlyAccess role, kubeconfig `view`) or a container
// without ~/.aws and ~/.kube (SECURITY).

// infraFamily is the guard of one tool, applied when the agent's own rules
// do not refuse probe.
type infraFamily struct {
	probe string
	read  []string // allowed
	ask   []string // asked (secrets, tunnels, repositories)
	keep  []string // allowed again after ask (harmless forms of an asked one)
}

var infraFamilies = []infraFamily{
	{probe: "terraform plan", read: []string{
		"terraform fmt*", "terraform validate*", "terraform init*", "terraform plan*", "terraform show*",
		"terraform state list*", "terraform state show*", "terraform providers*", "terraform graph*",
		"terraform workspace list*", "terraform workspace show*", "terraform version*", "terraform -version*",
	}, ask: []string{"*terraform* output*", "*terraform* console*"}},
	{probe: "tofu plan", read: []string{
		"tofu fmt*", "tofu validate*", "tofu init*", "tofu plan*", "tofu show*", "tofu state list*", "tofu state show*",
		"tofu providers*", "tofu graph*", "tofu workspace list*", "tofu workspace show*", "tofu version*",
	}, ask: []string{"*tofu* output*", "*tofu* console*"}},
	{probe: "kubectl get pods", read: []string{
		"kubectl get *", "kubectl describe *", "kubectl logs *", "kubectl explain *", "kubectl version*", "kubectl api-*",
		"kubectl top *", "kubectl diff *", "kubectl auth can-i *", "kubectl kustomize*", "kubectl config get-contexts*",
		"kubectl config current-context*", "kubectl cluster-info", "kubectl * --dry-run=client*", "kubectl * --dry-run=server*",
	}, ask: []string{"*kubectl* get secret*", "*kubectl* get secrets*", "*kubectl* describe secret*", "*kubectl* port-forward*", "*kubectl* proxy*"}},
	{probe: "helm list", read: []string{
		"helm template *", "helm lint *", "helm show *", "helm list*", "helm ls*", "helm status *", "helm history *",
		"helm diff *", "helm search *", "helm dependency list*", "helm dependency build*", "helm version*",
		"helm get manifest *", "helm get notes *",
	}, ask: []string{"*helm* get values*", "*helm* get all*", "*helm* repo add*", "*helm* repo update*"}},
	{probe: "aws s3 ls", read: []string{
		"aws sts get-caller-identity*", "aws * describe-*", "aws * list-*", "aws s3 ls*", "aws --version",
	}, ask: []string{"*aws* get-*"}, keep: []string{"aws sts get-caller-identity*"}},
	{probe: "gcloud projects list", read: []string{
		"gcloud * list*", "gcloud * describe *", "gcloud config list*", "gcloud version*", "gcloud info*",
	}},
	{probe: "az group list", read: []string{
		"az * list*", "az * show*", "az deployment * what-if *", "az deployment * validate *", "az version*", "az account show*",
	}},
	{probe: "pulumi preview", read: []string{
		"pulumi preview*", "pulumi stack ls*", "pulumi about*", "pulumi whoami*", "pulumi version*",
	}, ask: []string{"*pulumi* stack output*", "*pulumi* config*"}},
	{probe: "vault status", read: []string{
		"vault status*", "vault secrets list*", "vault kv list *", "vault kv metadata get *", "vault version*",
	}, ask: []string{"*vault* kv get*", "*vault* read *"}},
	{probe: "ansible-lint", read: []string{
		"ansible-lint*", "ansible-playbook * --syntax-check*", "ansible-playbook * --list-*", "ansible-inventory * --list*",
		"ansible --version*",
	}},
}

// ApplyLockPatterns are the infrastructure changes asked to an infra agent,
// and refused while `apply` is locked (leading `*`: global flags such as
// --context, --profile, -chdir may come first).
var ApplyLockPatterns = []string{
	"*terraform* apply*", "*terraform* destroy*", "*terraform* import*", "*terraform* refresh*",
	"*terraform* taint*", "*terraform* untaint*", "*terraform* state rm*", "*terraform* state mv*",
	"*terraform* state push*", "*terraform* state replace-provider*", "*terraform* force-unlock*",
	"*terraform* workspace new*", "*terraform* workspace delete*",
	"*tofu* apply*", "*tofu* destroy*", "*tofu* import*", "*tofu* refresh*", "*tofu* state rm*", "*tofu* state mv*",
	"*tofu* state push*", "*tofu* force-unlock*",
	"*kubectl* apply*", "*kubectl* create*", "*kubectl* delete*", "*kubectl* replace*", "*kubectl* patch*",
	"*kubectl* edit*", "*kubectl* scale*", "*kubectl* autoscale*", "*kubectl* rollout *", "*kubectl* drain*",
	"*kubectl* cordon*", "*kubectl* uncordon*", "*kubectl* taint*", "*kubectl* label*", "*kubectl* annotate*",
	"*kubectl* set *", "*kubectl* exec*", "*kubectl* cp *", "*kubectl* run *", "*kubectl* expose*", "*kubectl* debug*",
	"*kubectl* attach*", "*kubectl* certificate*", "*--dry-run=none*",
	"*helm* install*", "*helm* upgrade*", "*helm* uninstall*", "*helm* delete*", "*helm* rollback*", "*helm* test *",
	"*helm* plugin install*",
	"*pulumi* up*", "*pulumi* destroy*", "*pulumi* refresh*", "*pulumi* import*", "*pulumi* state *", "*pulumi* cancel*",
	"*aws* create-*", "*aws* delete-*", "*aws* put-*", "*aws* update-*", "*aws* modify-*", "*aws* terminate-*",
	"*aws* run-*", "*aws* start-*", "*aws* stop-*", "*aws* reboot-*", "*aws* attach-*", "*aws* detach-*",
	"*aws* invoke*", "*aws* deploy*", "*aws* send-*", "*aws* publish*", "*aws* tag-*", "*aws* untag-*",
	"*aws* s3 cp*", "*aws* s3 mv*", "*aws* s3 rm*", "*aws* s3 sync*", "*aws* s3 mb*", "*aws* s3 rb*",
	"*gcloud* create*", "*gcloud* delete*", "*gcloud* update*", "*gcloud* deploy*", "*gcloud* ssh*", "*gcloud* scp*",
	"*gcloud* set-iam-policy*", "*gcloud* add-iam-policy-binding*", "*gcloud* remove-iam-policy-binding*",
	"*az* create*", "*az* delete*", "*az* update*", "*az* deploy*", "*az* set *", "*az* start*", "*az* stop*", "*az* restart*",
	"*vault* write*", "*vault* kv put*", "*vault* kv patch*", "*vault* kv delete*", "*vault* kv destroy*", "*vault* delete*",
	"*vault* token*", "*vault* login*", "*vault* operator*", "*vault* policy write*", "*vault* secrets enable*",
}

// credentialDeny are the commands that print credentials: refused to every
// agent (global guard).
var credentialDeny = []string{
	"*get-secret-value*", "*--with-decryption*", "*get-login-password*", "*assume-role*", "*get-session-token*",
	"*print-access-token*", "*print-identity-token*", "*--show-secrets*", "*VAULT_TOKEN*", "*kubectl*config*view*--raw*",
	"*az* account get-access-token*", "*gh auth token*",
}

// InfraGuard returns the infrastructure rules of an agent (own: its rules;
// nothing for a tool its rules refuse).
func InfraGuard(own []sessionspec.PermissionRule) []sessionspec.PermissionRule {
	var shell []sessionspec.PermissionRule
	for _, r := range own {
		if r.Action == sessionspec.ActionShell {
			shell = append(shell, r)
		}
	}
	var out []sessionspec.PermissionRule
	add := func(p string, eff sessionspec.Effect) {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: eff})
	}
	found := false
	for _, f := range infraFamilies {
		if eff, ok := evaluate(shell, f.probe); !ok || eff == sessionspec.EffectDeny {
			continue
		}
		found = true
		if f.probe == "ansible-lint" {
			// A playbook run changes hosts (also with --check on some
			// modules): asked, its read-only forms allowed after.
			add("*ansible-playbook *", sessionspec.EffectAsk)
		}
		for _, p := range f.read {
			add(p, sessionspec.EffectAllow)
		}
		for _, p := range f.ask {
			add(p, sessionspec.EffectAsk)
		}
		for _, p := range f.keep {
			add(p, sessionspec.EffectAllow)
		}
	}
	if found {
		for _, p := range ApplyLockPatterns {
			add(p, sessionspec.EffectAsk)
		}
	}
	return out
}

// ApplyLockedByDefault returns the session rules that refuse the
// infrastructure changes in a workflow that declares no checkpoint
// unlocking `apply` (nothing would ever open them: they stay refused).
func ApplyLockedByDefault(wf *sessionspec.WorkflowRuntime) []sessionspec.PermissionRule {
	if _, ok := UnlockingCheckpoint(wf, sessionspec.UnlockApply); ok {
		return nil
	}
	out := make([]sessionspec.PermissionRule, 0, len(ApplyLockPatterns))
	for _, p := range ApplyLockPatterns {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectDeny})
	}
	return out
}
