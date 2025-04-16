import * as pulumi from "@pulumi/pulumi";
import { config } from "../config";

// This functions returns the function in string format, as this is what Zitadel needs
const updateIamUserExternalId = (
  accessToken: pulumi.Output<string>,
): pulumi.Output<string> =>
  accessToken.apply((token) => {
    // @ts-expect-error
    // This must be JavaScript, not typescript
    let stringifiedFn = String(function updateIamUserExternalId(ctx, api) {
      let http = require("zitadel/http");
      let logger = require("zitadel/log");
      let uuid = require("zitadel/uuid");

      const operation = `Operation Id: ${uuid.v4()} /`;

      try {
        logger.log(`${operation} Getting user data`);
        const user = ctx.v1.getUser();
        logger.log(
          `${operation} User id: ${user.id}, user email: ${user.human.email}`,
        );

        const reqBody = {
          user: {
            annotations: {
              "internal.iam.datumapis.com/zitadel-id": user.id,
            },
          },
          update_mask: "annotations",
        };

        logger.log(`${operation} User to update: ${JSON.stringify(reqBody)}`);

        const headers = {
          Authorization: `Bearer __ACCESS_TOKEN__`,
          "Content-Type": "application/json",
        };

        const res = http
          .fetch(
            `__API_URL__/v1alpha/users/${user.human.email}:setUserProviderId`,
            {
              method: "POST",
              body: reqBody,
              headers: headers,
            },
          )
          .json();

        logger.log(
          `${operation} Updated into IAM System. IAM system name: ${JSON.stringify(res.response.name)}`,
        );
      } catch (e) {
        // @ts-expect-error
        const error = `${operation} Error: ${e.message}`;
        logger.log(error);
        throw error;
      }
    });

    // Replace the placeholder with the actual token
    stringifiedFn = stringifiedFn.replace(/__ACCESS_TOKEN__/g, token);

    // Replace the placeholder with the actual iam system api url
    stringifiedFn = stringifiedFn.replace(
      /__API_URL__/g,
      config.iamSystem.apiUrl,
    );

    return stringifiedFn;
  });

export { updateIamUserExternalId };
