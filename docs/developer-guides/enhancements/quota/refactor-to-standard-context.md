# Refactor Plan: ClaimCreationPolicy to Use Standard Kubernetes Context Only

## Overview

This document outlines the plan to refactor the ClaimCreationPolicy implementation to rely only on standard Kubernetes admission plugin RequestInfo, removing all custom organization and user context that isn't available through standard Kubernetes APIs.

## Current State Analysis

### Current Context Sources

The implementation currently uses the following context data:

1. **UserContext** (from admission.Attributes.GetUserInfo()):
   - `Name`: Username from RequestInfo ✅ (standard)
   - `UID`: User UID from RequestInfo ✅ (standard)
   - `Groups`: User groups from RequestInfo ✅ (standard)
   - `Extra`: Additional attributes from RequestInfo ✅ (standard)

2. **OrganizationContext** (custom, not from RequestInfo):
   - `Name`: Extracted from object labels/annotations or namespace ❌
   - `Type`: Hardcoded as "Standard" ❌
   - `Tier`: Hardcoded as "standard" ❌
   - `Labels`: Empty map ❌
   - `Annotations`: Empty map ❌

3. **Standard Context** (from admission.Attributes):
   - `Namespace`: From RequestInfo ✅ (standard)
   - `GVK`: From RequestInfo ✅ (standard)
   - `Object`: The resource being created ✅ (standard)

### Files Requiring Changes

1. **cel_engine.go**: 
   - Remove OrganizationContext struct
   - Remove organization variables from CEL environment
   - Update EvaluationContext struct

2. **plugin.go**:
   - Remove OrganizationContext creation
   - Remove extractOrganizationName method
   - Update buildEvaluationContext method

3. **template_engine.go**:
   - Remove organization fields from TemplateContext
   - Update BuildTemplateContext method

4. **Policy CRD types** (if needed):
   - Update documentation to reflect available context

## Refactoring Plan

### Phase 1: Define New Context Model

#### Available Context (Standard Kubernetes Only)

```go
type EvaluationContext struct {
    // Object is the Kubernetes object being created
    Object *unstructured.Unstructured
    
    // User contains information from admission.UserInfo
    User UserContext
    
    // Namespace where the object is being created
    Namespace string
    
    // GVK is the GroupVersionKind of the object
    GVK schema.GroupVersionKind
    
    // RequestInfo provides additional request context
    Request RequestContext
}

type UserContext struct {
    // From standard admission.UserInfo
    Name   string              // Username
    UID    string              // User unique identifier
    Groups []string            // User group memberships
    Extra  map[string][]string // Additional user attributes
}

type RequestContext struct {
    // Operation being performed (CREATE, UPDATE, DELETE, CONNECT)
    Operation string
    
    // SubResource if applicable (e.g., "status", "scale")
    SubResource string
    
    // DryRun indicates if this is a dry-run request
    DryRun bool
}
```

### Phase 2: CEL Expression Variables

#### Available CEL Variables (After Refactor)

```yaml
# Object variables (unchanged)
object                    # The resource being created/updated
object.metadata.name      # Resource name
object.metadata.namespace # Resource namespace
object.metadata.labels    # Resource labels
object.metadata.annotations # Resource annotations
object.spec.*            # Resource spec fields

# User variables (unchanged)
user.name                # Username
user.uid                 # User UID
user.groups              # User groups (list)
user.extra               # Extra user attributes (map)

# Context variables (unchanged)
namespace                # Target namespace
gvk.group               # API group
gvk.version             # API version
gvk.kind                # Resource kind

# Request variables (new)
request.operation        # CREATE, UPDATE, DELETE, CONNECT
request.subResource     # Sub-resource being accessed
request.dryRun          # Is this a dry-run request

# REMOVED variables
# organization.name      # No longer available
# organization.type      # No longer available
# organization.tier      # No longer available
# organization.labels    # No longer available
# organization.annotations # No longer available
```

### Phase 3: Migration Strategy for Policies

#### Option 1: Use Object Labels/Annotations

Users can encode organizational data in resource labels/annotations:

```yaml
apiVersion: quota.miloapis.com/v1alpha1
kind: ClaimCreationPolicy
metadata:
  name: deployment-policy
spec:
  targetResource:
    apiVersion: "apps/v1"
    kind: "Deployment"
  resourceClaimTemplate:
    requests:
    - resourceType: "apps/Deployment"
      amountExpression: "object.spec.replicas || 1"
      dimensionExpressions:
        # Extract tier from object labels instead of organization context
        tier: "object.metadata.labels['tier'] || 'standard'"
        org: "object.metadata.labels['org'] || 'default'"
```

#### Option 2: Use User Groups for Tier/Type Detection

```yaml
dimensionExpressions:
  # Determine tier based on user groups
  tier: |
    'enterprise' in user.groups ? 'enterprise' :
    'premium' in user.groups ? 'premium' : 
    'standard'
```

#### Option 3: Use Namespace Conventions

```yaml
dimensionExpressions:
  # Extract org from namespace naming convention
  org: |
    namespace.startsWith('org-') ? 
    namespace.substring(4) : 
    'default'
```

### Phase 4: Implementation Changes

#### 1. Update cel_engine.go

```go
// Remove OrganizationContext struct entirely

// Update EvaluationContext
type EvaluationContext struct {
    Object    *unstructured.Unstructured
    User      UserContext
    Request   RequestContext  // New
    Namespace string
    GVK       schema.GroupVersionKind
}

// Update CEL environment creation
env, err := cel.NewEnv(
    // Object variables
    cel.Variable("object", cel.DynType),
    
    // User variables
    cel.Variable("user", cel.ObjectType("user")),
    cel.Variable("user.name", cel.StringType),
    cel.Variable("user.uid", cel.StringType),
    cel.Variable("user.groups", cel.ListType(cel.StringType)),
    cel.Variable("user.extra", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
    
    // Request variables (new)
    cel.Variable("request", cel.ObjectType("request")),
    cel.Variable("request.operation", cel.StringType),
    cel.Variable("request.subResource", cel.StringType),
    cel.Variable("request.dryRun", cel.BoolType),
    
    // Context variables
    cel.Variable("namespace", cel.StringType),
    cel.Variable("gvk", cel.ObjectType("gvk")),
    cel.Variable("gvk.group", cel.StringType),
    cel.Variable("gvk.version", cel.StringType),
    cel.Variable("gvk.kind", cel.StringType),
)
```

#### 2. Update plugin.go

```go
// Update buildEvaluationContext
func (p *ClaimCreationPlugin) buildEvaluationContext(attrs admission.Attributes, obj *unstructured.Unstructured) *EvaluationContext {
    user := UserContext{
        Name:   attrs.GetUserInfo().GetName(),
        UID:    attrs.GetUserInfo().GetUID(),
        Groups: attrs.GetUserInfo().GetGroups(),
        Extra:  attrs.GetUserInfo().GetExtra(),
    }
    
    request := RequestContext{
        Operation:   string(attrs.GetOperation()),
        SubResource: attrs.GetSubResource(),
        DryRun:      attrs.IsDryRun(),
    }

    return &EvaluationContext{
        Object:    obj,
        User:      user,
        Request:   request,
        Namespace: attrs.GetNamespace(),
        GVK: schema.GroupVersionKind{
            Group:   attrs.GetKind().Group,
            Version: attrs.GetKind().Version,
            Kind:    attrs.GetKind().Kind,
        },
    }
}

// Remove extractOrganizationName method entirely
```

#### 3. Update template_engine.go

```go
type TemplateContext struct {
    // Resource information
    ResourceName string
    Namespace    string
    Kind         string
    APIVersion   string
    
    // User information
    UserName   string
    UserUID    string
    UserGroups []string
    
    // Request information (new)
    Operation    string
    SubResource  string
    
    // Context information
    GVK          string
    RandomSuffix string
    Timestamp    string
    
    // REMOVED: Organization fields
}
```

### Phase 5: Documentation Updates

#### Update Policy Examples

Transform existing examples to use only standard context:

**Before:**
```yaml
dimensionExpressions:
  tier: "organization.tier"
  org: "organization.name"
```

**After:**
```yaml
dimensionExpressions:
  # Option 1: From object labels
  tier: "object.metadata.labels['tier'] || 'standard'"
  org: "object.metadata.labels['org'] || 'default'"
  
  # Option 2: From user groups
  tier: "'tier-enterprise' in user.groups ? 'enterprise' : 'standard'"
  
  # Option 3: From namespace convention
  org: "namespace.startsWith('org-') ? namespace.substring(4) : 'default'"
```

## Benefits of This Approach

1. **Standards Compliance**: Uses only standard Kubernetes admission plugin context
2. **Portability**: Works with any Kubernetes cluster without custom modifications
3. **Security**: No need to fetch additional data during admission
4. **Performance**: No additional API calls or lookups required
5. **Simplicity**: Cleaner implementation with fewer dependencies

## Migration Guide for Existing Policies

### For Policies Using organization.tier

**Old:**
```yaml
conditionExpression: "organization.tier == 'enterprise'"
```

**New Options:**
```yaml
# Option 1: Check user groups
conditionExpression: "'tier-enterprise' in user.groups"

# Option 2: Check object labels
conditionExpression: "object.metadata.labels['tier'] == 'enterprise'"

# Option 3: Check namespace labels (requires namespace to have labels)
conditionExpression: "namespace == 'enterprise-namespace'"
```

### For Policies Using organization.name

**Old:**
```yaml
dimensionExpressions:
  org: "organization.name"
```

**New Options:**
```yaml
# Option 1: Extract from namespace
dimensionExpressions:
  org: "namespace.startsWith('org-') ? namespace.substring(4) : namespace"

# Option 2: From object labels
dimensionExpressions:
  org: "object.metadata.labels['organization'] || 'default'"
```

## Testing Plan

1. **Unit Tests**: Update all CEL expression tests to use new context
2. **Integration Tests**: Verify policies work with standard context only
3. **Migration Tests**: Test migration of existing policies to new format
4. **Performance Tests**: Ensure no performance degradation

## Rollout Strategy

1. **Phase 1**: Update code to support both old and new context (backwards compatible)
2. **Phase 2**: Deprecate organization context with warnings
3. **Phase 3**: Remove organization context support entirely

## Timeline

- Week 1: Implement code changes
- Week 2: Update tests and documentation
- Week 3: Testing and validation
- Week 4: Release with deprecation notices