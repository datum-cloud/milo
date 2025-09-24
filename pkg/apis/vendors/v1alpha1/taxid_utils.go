package v1alpha1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetTaxIdFromSecret retrieves the tax ID from the referenced Secret
func GetTaxIdFromSecret(ctx context.Context, c client.Client, vendor *Vendor, taxIdRef TaxIdReference) (string, error) {
	// Determine the namespace for the Secret
	namespace := taxIdRef.Namespace
	if namespace == "" {
		namespace = vendor.Namespace
		if namespace == "" {
			namespace = "default" // fallback for cluster-scoped resources
		}
	}

	// Get the Secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      taxIdRef.SecretName,
		Namespace: namespace,
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, taxIdRef.SecretName, err)
	}

	// Extract the tax ID from the Secret
	taxIdBytes, exists := secret.Data[taxIdRef.SecretKey]
	if !exists {
		return "", fmt.Errorf("key %s not found in secret %s/%s", taxIdRef.SecretKey, namespace, taxIdRef.SecretName)
	}

	return string(taxIdBytes), nil
}

// ValidateTaxIdSecret validates that the referenced Secret exists and contains the expected key
func ValidateTaxIdSecret(ctx context.Context, c client.Client, vendor *Vendor, taxIdRef TaxIdReference) error {
	// Determine the namespace for the Secret
	namespace := taxIdRef.Namespace
	if namespace == "" {
		namespace = vendor.Namespace
		if namespace == "" {
			namespace = "default" // fallback for cluster-scoped resources
		}
	}

	// Get the Secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      taxIdRef.SecretName,
		Namespace: namespace,
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", namespace, taxIdRef.SecretName, err)
	}

	// Check if the key exists
	if _, exists := secret.Data[taxIdRef.SecretKey]; !exists {
		return fmt.Errorf("key %s not found in secret %s/%s", taxIdRef.SecretKey, namespace, taxIdRef.SecretName)
	}

	return nil
}

// CreateTaxIdSecret creates a Secret containing the tax ID
func CreateTaxIdSecret(ctx context.Context, c client.Client, vendor *Vendor, taxIdRef TaxIdReference, taxId string) error {
	// Determine the namespace for the Secret
	namespace := taxIdRef.Namespace
	if namespace == "" {
		namespace = vendor.Namespace
		if namespace == "" {
			namespace = "default" // fallback for cluster-scoped resources
		}
	}

	// Create the Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taxIdRef.SecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"vendor.miloapis.com/vendor": vendor.Name,
				"vendor.miloapis.com/type":   "tax-id",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			taxIdRef.SecretKey: []byte(taxId),
		},
	}

	// Set owner reference to the vendor
	secret.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: vendor.APIVersion,
			Kind:       vendor.Kind,
			Name:       vendor.Name,
			UID:        vendor.UID,
			Controller: &[]bool{true}[0],
		},
	}

	return c.Create(ctx, secret)
}

// UpdateTaxIdSecret updates an existing Secret containing the tax ID
func UpdateTaxIdSecret(ctx context.Context, c client.Client, vendor *Vendor, taxIdRef TaxIdReference, taxId string) error {
	// Determine the namespace for the Secret
	namespace := taxIdRef.Namespace
	if namespace == "" {
		namespace = vendor.Namespace
		if namespace == "" {
			namespace = "default" // fallback for cluster-scoped resources
		}
	}

	// Get the existing Secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      taxIdRef.SecretName,
		Namespace: namespace,
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", namespace, taxIdRef.SecretName, err)
	}

	// Update the tax ID
	secret.Data[taxIdRef.SecretKey] = []byte(taxId)

	return c.Update(ctx, secret)
}

// DeleteTaxIdSecret deletes the Secret containing the tax ID
func DeleteTaxIdSecret(ctx context.Context, c client.Client, vendor *Vendor, taxIdRef TaxIdReference) error {
	// Determine the namespace for the Secret
	namespace := taxIdRef.Namespace
	if namespace == "" {
		namespace = vendor.Namespace
		if namespace == "" {
			namespace = "default" // fallback for cluster-scoped resources
		}
	}

	// Delete the Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taxIdRef.SecretName,
			Namespace: namespace,
		},
	}

	return c.Delete(ctx, secret)
}
