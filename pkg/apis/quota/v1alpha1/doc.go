// Package v1alpha1 contains API schema definitions for the quota.miloapis.com group.
//
// # Quota System Overview
//
// The quota system provides comprehensive resource quota management with real‑time enforcement
// and policy‑driven automation. It consists of six main API types that work together to enable
// scalable quota management.
//
// # Core Resource Types
//
// - [ResourceRegistration](#resourceregistration): Defines which resource types the system manages with quota.
// Registers resource types (for example, Projects, Users) for quota tracking, defines measurement
// units and display formats, and specifies which resources can create claims against the registered type.
// - [ResourceGrant](#resourcegrant): Allocates quota to consumers (typically Organizations).
// Provides specific quota amounts for registered resource types, supports multi‑resource grants,
// and can be created manually or automatically by GrantCreationPolicy.
// - [ResourceClaim](#resourceclaim): Claims quota when resources are created.
// Automatically created by ClaimCreationPolicy admission webhooks. Provides real‑time
// quota enforcement during resource creation and links to the triggering resource for lifecycle alignment.
// - [AllowanceBucket](#allowancebucket): Tracks quota usage (system‑managed).
// Aggregates quota limits from multiple ResourceGrants, tracks consumption from ResourceClaims at scale,
// and provides real‑time availability calculations for admission decisions.
//
// # Policy Automation Types
//
// - [GrantCreationPolicy](#grantcreationpolicy): Automates ResourceGrant creation when trigger conditions are met.
// Uses CEL expressions and Go templates for dynamic content and supports cross‑cluster allocation.
// - [ClaimCreationPolicy](#claimcreationpolicy): Automates ResourceClaim creation for real‑time enforcement.
// Creates claims using a template when target resources are created.
//
// # Common Usage Pattern
//
// 1. Setup: Platform administrators create ResourceRegistrations and policies.
// 2. Provisioning: ResourceGrants allocate quota to consumers (manual or automated).
// 3. Enforcement: ResourceClaims provide real‑time quota enforcement.
// 4. Monitoring: AllowanceBuckets provide quota usage visibility.
//
// For end-to-end examples and task-oriented guides, see tutorials and how‑to guides (to be added).
//
// +k8s:deepcopy-gen=package,register
// +groupName=quota.miloapis.com
package v1alpha1
