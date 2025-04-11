import * as pulumi from "@pulumi/pulumi";

// This functions returns the function in string format, as this is what Zitadel needs
const addEmailClaimScript = (): string => {
  // @ts-expect-error
  // This must be JavaScript, not typescript
  let stringifiedFn = String(function addEmailClaim(ctx, api) {
    let logger = require("zitadel/log");
    let uuid = require("zitadel/uuid");

    const operation = `Operation Id: ${uuid.v4()} /`;

    try {
      const user = ctx.v1.getUser();

      let email;
      if (user?.human) {
        logger.log(`${operation} User is human, adding email claim`);
        email = user.human.email;
      } else {
        logger.log(
          `${operation} User is not human, adding username as email claim`,
        );
        email = user.username;
      }

      api.v1.claims.setClaim("email", email);
      logger.log(
        `${operation} Email claim ${email} added to user Id ${user.id}`,
      );
    } catch (e) {
      logger.log(`${operation} Failed to add email claim`);
      // @ts-expect-error
      const error = `${operation} Error: ${e.message}`;
      logger.log(error);
      throw error;
    }
  });

  return stringifiedFn;
};

export { addEmailClaimScript };
