package quota

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// extractConditions converts unstructured .status.conditions to []metav1.Condition
func extractConditions(u *unstructured.Unstructured) []metav1.Condition {
	sl, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil || !found {
		return nil
	}
	conds := make([]metav1.Condition, 0, len(sl))
	for _, ci := range sl {
		if m, ok := ci.(map[string]interface{}); ok {
			t, _, _ := unstructured.NestedString(m, "type")
			s, _, _ := unstructured.NestedString(m, "status")
			r, _, _ := unstructured.NestedString(m, "reason")
			msg, _, _ := unstructured.NestedString(m, "message")
			conds = append(conds, metav1.Condition{Type: t, Status: metav1.ConditionStatus(s), Reason: r, Message: msg})
		}
	}
	return conds
}
