import { Bricolage_Grotesque } from "next/font/google";

// Single typeface for the web app. Keep this as the only font definition so a
// future typeface change is a one-file edit.
export const bricolageGrotesque = Bricolage_Grotesque({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-bricolage-grotesque",
});
