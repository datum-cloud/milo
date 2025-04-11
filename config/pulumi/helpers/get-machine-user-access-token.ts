import * as pulumi from "@pulumi/pulumi";
import * as zitadel from "@pulumiverse/zitadel";
import * as jwt from "jsonwebtoken";
import { config } from "../config";

// This function retrieves the access token for a machine user in Zitadel.
const getMachineUserAccessToken = (
  machineUserkey: zitadel.MachineKey,
): pulumi.Output<string> => {
  const privateKeyDetails = machineUserkey.keyDetails.apply((keyDetails) => {
    const parsedKey = JSON.parse(keyDetails);
    return {
      kid: parsedKey.keyId as string,
      key: parsedKey.key as string,
      userId: parsedKey.userId as string,
    };
  });

  const signedJwt = pulumi
    .all([
      privateKeyDetails.kid,
      privateKeyDetails.key,
      privateKeyDetails.userId,
    ])
    .apply(([kid, key, userId]) => {
      const payLoad = {
        iss: userId,
        sub: userId,
        aud: [config.zitadel.url],
        exp: Math.floor(Date.now() / 1000) + 60 * 60, // 1 hour expiration
        iat: Math.floor(Date.now() / 1000), // Current time
      };

      const headers = {
        alg: "RS256",
        kid: kid,
      };

      return jwt.sign(payLoad, key, { algorithm: "RS256", header: headers });
    });

  const accessToken = signedJwt.apply(async (jwt) => {
    const response = await fetch(`${config.zitadel.url}/oauth/v2/token`, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: new URLSearchParams({
        grant_type: "urn:ietf:params:oauth:grant-type:jwt-bearer",
        scope: "urn:zitadel:iam:org:project:id:zitadel:aud",
        assertion: jwt,
      }),
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch access token: ${response.statusText}`);
    }

    const data = await response.json();
    return data.access_token as string;
  });

  return accessToken;
};

export { getMachineUserAccessToken };
