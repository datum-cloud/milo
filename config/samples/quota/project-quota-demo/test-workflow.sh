#!/bin/bash

# Project Quota Demo Test Script
# This script demonstrates the complete project quota management workflow

set -e

echo "🚀 Starting Project Quota Demo"
echo "==============================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to wait for user input
wait_for_user() {
    echo -e "${YELLOW}Press Enter to continue...${NC}"
    read -r
}

# Function to apply and wait
apply_and_wait() {
    local file=$1
    local wait_time=${2:-5}
    echo -e "${BLUE}Applying $file...${NC}"
    kubectl apply -f "$file"
    echo "Waiting ${wait_time}s for resource to be processed..."
    sleep $wait_time
}

# Function to check resource status
check_status() {
    local resource_type=$1
    local name=$2
    local namespace=${3:-""}
    
    if [ -n "$namespace" ]; then
        kubectl get "$resource_type" "$name" -n "$namespace" -o wide 2>/dev/null || echo -e "${RED}Resource $resource_type/$name not found in namespace $namespace${NC}"
    else
        kubectl get "$resource_type" "$name" -o wide 2>/dev/null || echo -e "${RED}Resource $resource_type/$name not found${NC}"
    fi
}

echo -e "${GREEN}Step 1: Apply ResourceRegistration${NC}"
echo "This defines the quota resource type for projects per organization."
apply_and_wait "01-resource-registration.yaml" 3
check_status "resourceregistration" "projects-per-organization-registration"
wait_for_user

echo -e "${GREEN}Step 2: Apply ResourceGrant${NC}"
echo "This provides quota allowances for the acme-corp organization."
apply_and_wait "02-resource-grant.yaml" 3
check_status "resourcegrant" "acme-corp-project-quota-grant" "milo-system"
wait_for_user

echo -e "${GREEN}Step 3: Apply ClaimCreationPolicy${NC}"
echo "This enables automatic ResourceClaim creation and quota enforcement."
apply_and_wait "03-claim-creation-policy.yaml" 5
check_status "claimcreationpolicy" "project-quota-enforcement-policy"

echo -e "${BLUE}Waiting for ClaimCreationPolicy to become Ready...${NC}"
sleep 3
kubectl get claimcreationpolicies project-quota-enforcement-policy -o wide
wait_for_user

echo -e "${GREEN}Step 4: Create test organization${NC}"
echo "This creates the acme-corp organization that will receive quota."
apply_and_wait "04-test-organization.yaml" 3
check_status "organization" "acme-corp"
wait_for_user

echo -e "${GREEN}Step 5: Test project creation with quota enforcement${NC}"
echo "We'll create projects one by one to demonstrate quota limits."

# Extract individual projects from the test file
projects=(
    "acme-dev-api"
    "acme-dev-web"
    "acme-dev-mobile"
    "acme-standard-api" 
    "acme-standard-web"
    "acme-prod-api"
    "acme-unlabeled-project"
)

project_descriptions=(
    "Development tier, us-east-1 (should succeed)"
    "Development tier, us-east-1 (should succeed)" 
    "Development tier, us-east-1 (should FAIL - quota exceeded)"
    "Standard tier, us-east-1 (should succeed)"
    "Standard tier, us-west-2 (should succeed)"
    "Production tier, us-east-1 (should succeed)"
    "No quota labels (should FAIL - condition not met)"
)

for i in "${!projects[@]}"; do
    project_name=${projects[$i]}
    description=${project_descriptions[$i]}
    
    echo -e "${BLUE}Creating project: $project_name${NC}"
    echo -e "${YELLOW}Expected: $description${NC}"
    
    # Extract the project manifest and apply it
    if kubectl apply -f <(kubectl create --dry-run=client -o yaml -f 05-test-projects.yaml | yq eval "select(.metadata.name == \"$project_name\")"); then
        echo -e "${GREEN}✅ Project $project_name created successfully${NC}"
        
        # Show the ResourceClaim that was created
        echo "Checking for ResourceClaim..."
        sleep 2
        kubectl get resourceclaims -n milo-system --show-labels | grep "$project_name" || echo "No ResourceClaim found yet"
        
    else
        echo -e "${RED}❌ Project $project_name creation failed (quota enforced)${NC}"
    fi
    
    echo ""
    wait_for_user
done

echo -e "${GREEN}Step 6: Review final state${NC}"
echo "Let's examine what was created:"

echo -e "${BLUE}Projects:${NC}"
kubectl get projects -o wide | grep acme || echo "No acme projects found"

echo -e "${BLUE}ResourceClaims:${NC}"
kubectl get resourceclaims -n milo-system -o wide

echo -e "${BLUE}ResourceGrant status:${NC}"
kubectl get resourcegrant acme-corp-project-quota-grant -n milo-system -o yaml | grep -A10 status:

echo ""
echo -e "${GREEN}✅ Project Quota Demo Complete!${NC}"
echo "================================================"
echo "This demonstration showed:"
echo "1. How to register quota resource types"
echo "2. How to grant quota allowances to organizations"  
echo "3. How to enforce quota limits through admission control"
echo "4. How ResourceClaims are automatically created"
echo "5. How quota limits prevent resource creation when exceeded"