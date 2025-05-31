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
            "internal.iam.miloapis.com/zitadel-id": "pending",
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

        const postResponse  = http
        .fetch("__API_URL__/v1alpha/users", {
          method: "POST",
          body: reqBody,
          headers: headers,
        })

        const createUserResponse = postResponse.json();

        // If the user already exists
        if(postResponse.status == 409) {
          logger.log(`${operation} User already exists in IAM system.`);
          logger.log(`${operation} Getting existing user from IAM system.`);

          const getResponse = http
          .fetch(`__API_URL__/v1alpha/users/${reqBody.spec.email}`, {
            method: "GET",
            headers: headers,
          })

          const getUserResponse = getResponse.json();

          if(getResponse.status < 200 || getResponse.status >= 300) {
            throw new Error(`${operation} Failed to get user from IAM system. Status: ${getResponse.status}. Message: ${getUserResponse.message}`);
          }

          logger.log(`${operation} IAM System user resource name: ${user.name}`);
          logger.log(`${operation} Updating Zitadel user metadata with IAM System user resource name: ${user.name}`);

          api.v1.user.appendMetadata(
            "internal.iam.miloapis.com-iam-resource-name",
            getUserResponse.name,
          );

          return;
        }

        if(postResponse.status < 200 || postResponse.status >= 300) {
          throw new Error(`${operation} Failed to create user in IAM system. Status: ${postResponse.status}. Message: ${createUserResponse.message}`);
        }

        logger.log(
          `${operation} User Created into IAM System. IAM system name: ${createUserResponse.response.name}`,
        );

        api.v1.user.appendMetadata(
          "internal.iam.miloapis.com-iam-resource-name",
          createUserResponse.response.name,
        );
      } catch (e) {
        // @ts-expect-error
        const error = `${e.message}`;
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
