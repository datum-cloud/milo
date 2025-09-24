// Package v1alpha1 contains API schema definitions for the quota.miloapis.com group.
//
// # Quota System Overview
//
// The quota system enables platform administrators to control resource consumption
// through real-time enforcement and automated policy execution. The system tracks
// resource usage, allocates capacity to consumers, and prevents resource creation
// when limits are exceeded.
//
// # Core Resource Types
//
// The quota system uses four core types that manage resource tracking and allocation:
//
// **[ResourceRegistration](#resourceregistration)**: Registers a resource type for quota tracking.
// Platform administrators create registrations to define measurement units, display formats,
// and specify which resources can consume the tracked resource type. For example, registering
// "Projects per Organization" allows the system to track Project creation within Organizations.
//
// **[ResourceGrant](#resourcegrant)**: Allocates quota capacity to a specific consumer.
// Grants provide concrete allowances (for example, "100 Projects") to consumers like Organizations.
// Multiple grants for the same consumer and resource type combine to determine total capacity.
// Administrators create grants manually or automate them using GrantCreationPolicy.
//
// **[ResourceClaim](#resourceclaim)**: Requests quota during resource creation.
// Claims consume allocated capacity when resources are created. The system evaluates
// each claim against available quota and either grants or denies the request.
// ClaimCreationPolicy typically creates claims automatically during admission.
//
// **[AllowanceBucket](#allowancebucket)**: Aggregates quota limits and usage for decision-making.
// The system creates one bucket per consumer-resource type combination. Buckets combine
// capacity from all active ResourceGrants and track consumption from all granted ResourceClaims
// to calculate real-time availability for admission decisions.
//
// # Policy Automation Types
//
// Two policy types automate quota management based on resource lifecycle events:
//
// **[GrantCreationPolicy](#grantcreationpolicy)**: Creates ResourceGrants when conditions are met.
// Policies watch for resource changes and automatically provision quota capacity.
// For example, automatically grant "100 Projects" when a new Organization is created.
// Supports cross-cluster allocation and CEL expression-based conditions.
//
// **[ClaimCreationPolicy](#claimcreationpolicy)**: Creates ResourceClaims during admission.
// Policies intercept resource creation requests and generate quota claims for evaluation.
// For example, create a "1 Project" claim when a Project resource is created.
// Uses Go templates for dynamic claim content and CEL expressions for trigger conditions.
//
// # How the System Works
//
// The quota system follows this workflow:
//
// 1. **Registration**: Administrators register resource types that require quota tracking.
// 2. **Allocation**: ResourceGrants provide quota capacity to consumers.
// 3. **Aggregation**: AllowanceBuckets combine grants and track usage for each consumer-resource pair.
// 4. **Enforcement**: ResourceClaims request quota during admission; the system grants or denies based on bucket availability.
// 5. **Monitoring**: Bucket status provides real-time quota usage visibility.
//
// # Policy Automation Workflow
//
// Policies automate the allocation and enforcement steps:
//
// 1. **Grant Policies**: Monitor resource changes and create grants when conditions match.
// 2. **Claim Policies**: Intercept admission requests and create claims for quota enforcement.
// 3. **Evaluation**: The system processes claims against bucket capacity in real-time.
//
// # Status and Conditions
//
// All resource types use standard Kubernetes status conditions to communicate state:
//
// - **Active/Ready conditions**: Indicate when resources are operational and contributing to quota decisions.
// - **Validation conditions**: Report configuration errors and resolution guidance.
// - **ObservedGeneration**: Tracks which specification version the controller has processed.
//
// Controllers update status conditions to reflect current state and provide troubleshooting information
// when problems occur.
//
// +k8s:deepcopy-gen=package,register
// +groupName=quota.miloapis.com
package v1alpha1
