<p align="center"><img src="docs/images/milo.png" width="500px"></p>

# Milo

Milo is a "business operating system" for product-led, B2B companies. Think of
it like a control plane for modern service providers, built on top of a
comprehensive system of record that ties together key parts of your business.

## Why We're Building Milo

Over the last two decades scaling infrastructure clouds (Voxel, Packet,
SoftLayer, StackPath), we've spent a lot of time building or stitching together
the pieces required to run a company at some decent scale: understanding our
contacts, users, accounts, usage, quotes, contracts, agreements, billing, etc.

While a number of awesome vertical tools have emerged to solve particular pain
points (like authorization, metering, or SOC2 compliance), fast-growing
companies have a large "back office" surface area to maintain and very little
go-to-market (GTM) tooling suitable for complex motions. Inevitably, each tool
needs foundational, trusted data upon which to act, creating a competing "system
of record" environment. The recent emergence of AI agents makes this even more
clear.

As we set out to build [Datum Cloud](https://www.datum.net) (an infrastructure
cloud optimized for network and data sensitive workloads), we were driven to
help a new class of service providers gain hyperscaler advantages. We decided
that instead of simply using the lessons we'd learned over the years to build
our own kick-butt back office, we should make it available to others. Et voila!

## What We Prefer Not to Build

Projects with such a wide surface area can engender a "build everything"
mindset. While our vision calls for a pretty comprehensive approach, there are a
number of capabilities that are either commoditized or serviced by existing
scaled vendors. Here are some examples:

- Email sending can be provided by Twilio, Resend, etc.
- Authentication can be provided by Zitadel, Clerk, Auth0, Descope, etc.
- General automation can be provided by Zapier, Workato, Make, etc
- Product analytics and visualization can be provided by PostHog, Grafana, etc.
- User enrichment can be provided by Clay, Apollo, Clearbit, etc.
- Payments can be provided by Stripe, Adyen, etc.
- Tax and financial compliance can be provided by Avalara, NetSuite, etc.

## What We're Starting With
There are a few big "System of Record" buckets to which we think folks should
have programmatic access, namely: contacts, accounts, products, vendors, and
assets.

- Operator Portal: Hosted admin panel for a "single pane of glass" business
  view.
- Contacts: Marketing contacts management with dynamic lists and opt-in.
- Customers: User, Account, Parent Account management w/ standard workflows.
- Staff Management: A source of truth for RBAC and related workflows.
- Vendor Profiles: Supplier profiles with related documents.
- Fraud & Abuse: Basic risk and fraud scoring for user sign ups
- Agreements: Online and offline management of AuP, ToS, MSA, NDA, etc.
- Audit Logs: Unified cross platform event and audit logs.
- Product Catalog: Programmable foundation for billing, quoting, feature access.
- Pricing: Transparent pricing models tailored for scalability and flexibility.
- Entitlements: Management of feature access, quotas and tiering.

## Running the APIServer Locally

To get started with Milo's APIServer locally, follow these steps:

### 1. Create a local Kubernetes cluster with kind

```sh
kind create cluster --name kind-etcd --config dev/kind-etcd-port.yaml
```

### 2. Run the Milo APIServer

Make sure you have the required certificates and configuration files in place. This certificates should be automatically populated by the previous command. Then run:

```sh
go run ./cmd/milo/main.go apiserver \
  --etcd-servers=https://127.0.0.1:2380 \
  --etcd-cafile=$(pwd)/dev/.kind-etcd-certs/etcd/ca.crt \
  --etcd-certfile=$(pwd)/dev/.kind-etcd-certs/apiserver-etcd-client.crt \
  --etcd-keyfile=$(pwd)/dev/.kind-etcd-certs/apiserver-etcd-client.key \
  --service-account-issuer=https://kubernetes.default.svc.cluster.local \
  --service-account-signing-key-file=$(pwd)/dev/service-account-key.pem \
  --service-account-key-file=$(pwd)/dev/service-account-key.pem \
  --token-auth-file=./dev/token.csv \
  --authorization-mode=RBAC \
  --storage-media-type=application/json \
  --kubeconfig ~/.kube/config
```

### 3. Check for installed API types at runtime

To see which custom resources are available in the running API server:

```sh
kubectl --server=https://127.0.0.1:6443 --insecure-skip-tls-verify \
        --token=mytoken \
        api-resources --api-group=iam.miloapis.com
```

### 4. Apply an example resource

You can create a resource (e.g., a MachineAccountKey) using:

```sh
kubectl apply -f config/samples/iam/v1alpha1/machineaccountkey.yaml \
       --server=https://127.0.0.1:6443 \
       --insecure-skip-tls-verify \
       --token=mytoken
```

## Future Capabilities

We see integrated commercial functionality as the big unlock for scale. Here are
some areas we're planning to work on:

- Privacy: GDPR policy management with sub-processor and change notifications
- Deal Rooms: Hosted trust centers for quotes, policies, & agreements
- Quoting: Generate and manage quotes for streamlined sales engagements.
- Order Management: A workflow for contract and order lifecycle management.
- Purchase Orders: An API to support end user procurement tracking
- Billing: A centralized hub for account statements, two sided ledger, & invoice
  status

