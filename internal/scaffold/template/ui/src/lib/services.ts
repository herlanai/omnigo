import { OmniWebClient } from "./omniweb";

// Explicit sample contract. Keep it aligned with app/main.go; omni generate can
// generate contracts from root-package service registrations in larger apps.
type Services = {
  "app.greet": { input: { name: string }; output: { message: string } };
};
export const services = new OmniWebClient<Services>();
