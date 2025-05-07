import * as pulumi from "@pulumi/pulumi";
import { config } from "../config";

// This functions returns the function in string format, as this is what Zitadel needs
const createIamUserScript = (
  accessToken: pulumi.Output<string>,
): pulumi.Output<string> =>
  accessToken.apply((token) => {
    // @ts-expect-error
    // This must be JavaScript, not typescript
    let stringifiedFn = String(function createIamUser(ctx, api) {
      let http = require("zitadel/http");
      let logger = require("zitadel/log");
      let uuid = require("zitadel/uuid");

      const operation = `Operation Id: ${uuid.v4()} /`;

      try {
        logger.log(`${operation} Getting user information`);
        if (!ctx?.v1?.user?.human) {
          throw new Error(`${operation} User data is missing or incomplete.`);
        }

        const user = ctx.v1.user.human;

        const reqBody = {
          display_name: user.displayName || "",
          annotations: {
            "internal.iam.datumapis.com/zitadel-id": "pending",
          },
          spec: {
            email: user.email,
            given_name: user.firstName || "",
            family_name: user.lastName || "",
          },
        };

        logger.log(`${operation} User to create: ${JSON.stringify(reqBody)}`);

        const headers = {
          Authorization: `Bearer __ACCESS_TOKEN__`,
          "Content-Type": "application/json",
        };

        var res = http
          .fetch("__API_URL__/v1alpha/users", {
            method: "POST",
            body: reqBody,
            headers: headers,
          })
          .json();

        if(res.code == 6) {
          // TODO: As the user already exists on the database, we wil need to
          // update the metadata with the corresponding iam resource name later
          logger.log(`${operation} User already exists in IAM system.`);
        
          api.v1.user.appendMetadata(
            "internal.iam.datumapis.com-iam-resource-name",
            "pending",
          );

          return;
        }

        logger.log(
          `${operation} User Created into IAM System. IAM system name: ${res.response.name}`,
        );

        api.v1.user.appendMetadata(
          "internal.iam.datumapis.com-iam-resource-name",
          res.response.name,
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

export { createIamUserScript };
