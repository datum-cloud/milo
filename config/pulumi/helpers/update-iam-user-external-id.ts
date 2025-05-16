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
          provider_id: user.id,
        };

        logger.log(`${operation} User to update: ${JSON.stringify(reqBody)}`);

        const headers = {
          Authorization: `Bearer __ACCESS_TOKEN__`,
          "Content-Type": "application/json",
        };


        const response = http
        .fetch(
          `__API_URL__/v1alpha/users/${user.human.email}:setUserProviderId`,
          {
            method: "POST",
            body: reqBody,
            headers: headers,
          },
        )

        const setUserProviderIdResponse = response.json();

        if(response.status < 200 || response.status >= 300) {
          throw new Error(`${operation} Failed to update user provider id in IAM system. Status: ${response.status}. Message: ${setUserProviderIdResponse.message}`);
        }

        logger.log(
          `${operation} Updated into IAM System. IAM system name: ${JSON.stringify(setUserProviderIdResponse.user.name)}`,
        );
      } catch (e) {
        // @ts-expect-error
        const error = e.message;
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
