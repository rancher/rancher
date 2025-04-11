package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	ErrUnsupportedEncoding = fmt.Errorf("unsupported encoding")
)

type Policy struct {
	Filters   []*Filter
	Redactors []Redactor
	Verbosity auditlogv1.LogVerbosity
}

func (p Policy) actionForLog(log *log) auditlogv1.FilterAction {
	if len(p.Filters) == 0 {
		return auditlogv1.FilterActionAllow
	}

	for _, f := range p.Filters {
		if f.Allowed(log) {
			return auditlogv1.FilterActionAllow
		}
	}

	return auditlogv1.FilterActionDeny
}

func PolicyFromAuditPolicy(policy *auditlogv1.AuditPolicy) (Policy, error) {
	newPolicy := Policy{
		Filters:   make([]*Filter, len(policy.Spec.Filters)),
		Redactors: make([]Redactor, len(policy.Spec.AdditionalRedactions)),
		Verbosity: policy.Spec.Verbosity,
	}

	if newPolicy.Verbosity.Level != auditlogv1.LevelNull {
		newPolicy.Verbosity = verbosityForLevel(newPolicy.Verbosity.Level)
	}

	for i, f := range policy.Spec.Filters {
		switch f.Action {
		case auditlogv1.FilterActionAllow, auditlogv1.FilterActionDeny:
		default:
			return Policy{}, fmt.Errorf("failed to create filter: invalid filter action: '%s'", f.Action)
		}

		filter, err := NewFilter(f)
		if err != nil {
			return Policy{}, fmt.Errorf("failed to create filter: %w", err)
		}

		newPolicy.Filters[i] = filter
	}

	for i, r := range policy.Spec.AdditionalRedactions {
		redactor, err := NewRedactor(r)
		if err != nil {
			return Policy{}, fmt.Errorf("failed to create redactor: %w", err)
		}

		newPolicy.Redactors[i] = redactor
	}

	return newPolicy, nil
}

type WriterOptions struct {
	DefaultPolicyLevel auditlogv1.Level

	DisableDefaultPolicies bool
}

type Writer struct {
	WriterOptions

	policiesMutex sync.RWMutex
	policies      map[types.NamespacedName]Policy

	output io.Writer
}

func NewWriter(output io.Writer, opts WriterOptions) (*Writer, error) {
	w := &Writer{
		WriterOptions: opts,

		policies: make(map[types.NamespacedName]Policy),
		output:   output,
	}

	if !opts.DisableDefaultPolicies {
		for _, v := range DefaultPolicies() {
			if err := w.UpdatePolicy(&v); err != nil {
				return nil, fmt.Errorf("failed to add default policies: %w", err)
			}
		}
	}

	return w, nil
}

func (w *Writer) Write(log *log) error {
	redactors := []Redactor{}
	if !w.DisableDefaultPolicies {
		redactors = append(redactors, defaultRedactors...)
	}

	verbosity := verbosityForLevel(w.DefaultPolicyLevel)
	action := auditlogv1.FilterActionUnknown

	w.policiesMutex.RLock()
	for _, policy := range w.policies {
		switch policy.actionForLog(log) {
		case auditlogv1.FilterActionAllow:
			redactors = append(redactors, policy.Redactors...)
			verbosity = mergeLogVerbosities(verbosity, policy.Verbosity)

			action = auditlogv1.FilterActionAllow
		case auditlogv1.FilterActionDeny:
			if action != auditlogv1.FilterActionAllow {
				action = auditlogv1.FilterActionDeny
			}
		}
	}
	w.policiesMutex.RUnlock()

	if action == auditlogv1.FilterActionDeny {
		return nil
	}

	log.prepare(verbosity)

	for _, r := range redactors {
		if err := r.Redact(log); err != nil {
			return fmt.Errorf("failed to redact log: %w", err)
		}
	}

	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	var buffer bytes.Buffer
	if err := json.Compact(&buffer, data); err != nil {
		return fmt.Errorf("failed to compact log: %w", err)
	}
	buffer.WriteByte('\n')

	if _, err := w.output.Write(buffer.Bytes()); err != nil {
		return fmt.Errorf("failed to write log: %w", err)
	}

	return nil
}

func (w *Writer) UpdatePolicy(policy *auditlogv1.AuditPolicy) error {
	newPolicy, err := PolicyFromAuditPolicy(policy)
	if err != nil {
		return err
	}

	name := types.NamespacedName{
		Name:      policy.Name,
		Namespace: policy.Namespace,
	}

	w.policiesMutex.Lock()
	w.policies[name] = newPolicy
	w.policiesMutex.Unlock()

	return nil
}

func (w *Writer) RemovePolicy(policy *auditlogv1.AuditPolicy) bool {
	w.policiesMutex.Lock()
	defer w.policiesMutex.Unlock()

	name := types.NamespacedName{
		Name:      policy.Name,
		Namespace: policy.Namespace,
	}

	if _, ok := w.policies[name]; ok {
		delete(w.policies, name)
		return true
	}

	return false
}

func (w *Writer) GetPolicy(namespace string, name string) (Policy, bool) {
	w.policiesMutex.RLock()
	defer w.policiesMutex.RUnlock()

	p, ok := w.policies[types.NamespacedName{Name: name, Namespace: namespace}]

	return p, ok
}

func (l *Writer) Start(ctx context.Context) {
	if l == nil {
		return
	}

	go func() {
		<-ctx.Done()
	}()
}
