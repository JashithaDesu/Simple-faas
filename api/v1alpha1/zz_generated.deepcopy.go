package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// NOTE: In a normal Kubebuilder project these are generated automatically
// by `make manifests` / controller-gen. Hand-written here since this
// sandbox has no network access to run codegen. Regenerate properly with
// controller-gen once you're on your own machine — see README.

func (in *Function) DeepCopyInto(out *Function) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	if in.Spec.Command != nil {
		out.Spec.Command = append([]string{}, in.Spec.Command...)
	}
if in.Spec.TriggeredReplicas != nil {
		tr := *in.Spec.TriggeredReplicas
		out.Spec.TriggeredReplicas = &tr
	}
	in.Status.DeepCopyInto(&out.Status)
}

func (in *Function) DeepCopy() *Function {
	if in == nil {
		return nil
	}
	out := new(Function)
	in.DeepCopyInto(out)
	return out
}

func (in *Function) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FunctionStatus) DeepCopyInto(out *FunctionStatus) {
	*out = *in
	if in.LastRequestTime != nil {
		out.LastRequestTime = in.LastRequestTime.DeepCopy()
	}
}

func (in *FunctionList) DeepCopyInto(out *FunctionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Function, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *FunctionList) DeepCopy() *FunctionList {
	if in == nil {
		return nil
	}
	out := new(FunctionList)
	in.DeepCopyInto(out)
	return out
}

func (in *FunctionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
