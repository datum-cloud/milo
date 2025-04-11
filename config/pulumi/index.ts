import * as zitadel from "@pulumiverse/zitadel";
import { getMachineUserAccessToken } from "./helpers/get-machine-user-access-token";
import { createIamUserScript } from "./helpers/create-iam-user-script";
import { config } from "./config";
import { addEmailClaimScript } from "./helpers/add-email-claim-script";
import { updateIamUserExternalId } from "./helpers/update-iam-user-external-id";

const org = new zitadel.Org("test-org", {
  name: "Datum Staging",
  isDefault: true,
});

const project = new zitadel.Project("default", {
  name: "IAM System",
  orgId: org.id,
  projectRoleAssertion: false,
  projectRoleCheck: false,
  hasProjectCheck: false,
  privateLabelingSetting:
    "PRIVATE_LABELING_SETTING_ENFORCE_PROJECT_RESOURCE_OWNER_POLICY",
});

const userCreatorMachineUser = new zitadel.MachineUser("default", {
  orgId: org.id,
  accessTokenType: "ACCESS_TOKEN_TYPE_JWT",
  userName: "usercreator@datum.com",
  name: "name",
  description: "a machine user for creating users into iam system",
  withSecret: false,
});

const userCreatorMachineUserKey = new zitadel.MachineKey("default", {
  orgId: org.id,
  userId: userCreatorMachineUser.id,
  keyType: "KEY_TYPE_JSON",
  expirationDate: "2519-04-01T08:45:00Z",
});

const addEmailClaimAction = new zitadel.Action("default_claim", {
  orgId: org.id,
  name: "addEmailClaim",
  script: addEmailClaimScript(), // Pass the resolved token
  timeout: "10s",
  allowedToFail: true,
});

const addEmailClaimActionTriggerAction = new zitadel.TriggerActions(
  "default_claim",
  {
    orgId: org.id,
    flowType: "FLOW_TYPE_CUSTOMISE_TOKEN",
    triggerType: "TRIGGER_TYPE_PRE_ACCESS_TOKEN_CREATION",
    actionIds: [addEmailClaimAction.id],
  },
);

const accessToken = getMachineUserAccessToken(userCreatorMachineUserKey);

const createIamUserAction = new zitadel.Action("default", {
  orgId: org.id,
  name: "createIamUser",
  script: createIamUserScript(accessToken), // Pass the resolved token
  timeout: "10s",
  allowedToFail: false,
});

const createIamUserTriggerAction = new zitadel.TriggerActions("default", {
  orgId: org.id,
  flowType: "FLOW_TYPE_EXTERNAL_AUTHENTICATION",
  triggerType: "TRIGGER_TYPE_PRE_CREATION",
  actionIds: [createIamUserAction.id],
});

const updateIamUserAction = new zitadel.Action("default_update_id", {
  orgId: org.id,
  name: "updateIamUserExternalId",
  script: updateIamUserExternalId(accessToken), // Pass the resolved token
  timeout: "10s",
  allowedToFail: false,
});

const updateIamUserTriggerAction = new zitadel.TriggerActions(
  "default_update_id",
  {
    orgId: org.id,
    flowType: "FLOW_TYPE_EXTERNAL_AUTHENTICATION",
    triggerType: "TRIGGER_TYPE_POST_CREATION",
    actionIds: [updateIamUserAction.id],
  },
);

const googleIdp = new zitadel.IdpGoogle("default", {
  name: "Google",
  clientId: config.googleIdp.clientId,
  clientSecret: config.googleIdp.clientSecret,
  scopes: ["openid", "profile", "email"],
  isLinkingAllowed: false,
  isCreationAllowed: true,
  isAutoCreation: false,
  isAutoUpdate: false,
});

const _member = new zitadel.OrgMember("default", {
  orgId: org.id,
  userId: userCreatorMachineUser.id,
  roles: ["ORG_OWNER"],
});

export { accessToken };
