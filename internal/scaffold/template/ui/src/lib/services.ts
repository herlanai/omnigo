import { OmniWebClient } from "./omniweb";

// Explicit contract until Go-to-TypeScript generation is implemented.
type Services = {
  "app.greet": { input: { name: string }; output: { message: string } };
};
export const services = new OmniWebClient<Services>();
