import * as pulumi from "@pulumi/pulumi";

const iamSystemConfig = new pulumi.Config("datum-iam-system");
const zitadelConfig = new pulumi.Config("zitadel-auth-provider");
const googleIdpConfig = new pulumi.Config("google-idp");

const config = {
  iamSystem: {
    apiUrl: iamSystemConfig.require("api-url"),
  },
  zitadel: {
    url: zitadelConfig.require("url"),
  },
  googleIdp: {
    clientId: googleIdpConfig.require("client-id"),
    clientSecret: googleIdpConfig.requireSecret("client-secret"),
  },
};

export { config };
