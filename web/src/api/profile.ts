import { get, put } from "./client";
import type { PersonalProfile } from "@/types/profile";

export function getPersonalProfile() {
  return get<PersonalProfile>("/api/fkteams/profile");
}

export function savePersonalProfile(profile: PersonalProfile) {
  return put<PersonalProfile>("/api/fkteams/profile", profile);
}
