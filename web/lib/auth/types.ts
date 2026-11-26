export type Photographer = {
  id: string;
  email: string;
  business_name: string;
  slug: string;
  plan_id: string;
};

export type AuthError = {
  error: string;
  code?: string;
};
