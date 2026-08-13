"use client";

import { motion, AnimatePresence } from "framer-motion";
import { type ReactNode } from "react";

/**
 * Page transition wrapper — soft fade + small translate.
 * Preserves scroll position when appropriate.
 * No dramatic transitions.
 */
export function PageTransition({ children }: { children: ReactNode }) {
  return (
    <AnimatePresence mode="wait">
      <motion.div
        key="page"
        initial={{ opacity: 0, y: 4 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0, y: -4 }}
        transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}

/**
 * Staggered list item transition for lists/tables
 */
export function StaggeredItem({ children, index = 0 }: { children: ReactNode; index: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1], delay: index * 0.02 }}
    >
      {children}
    </motion.div>
  );
}

/**
 * Fade in/out for modals, sheets, dialogs
 */
export function FadeTransition({ children, show }: { children: ReactNode; show: boolean }) {
  return (
    <motion.div
      initial={false}
      animate={show ? { opacity: 1 } : { opacity: 0 }}
      exit={show ? undefined : { opacity: 0 }}
      transition={{ duration: 0.15, ease: [0.16, 1, 0.3, 1] }}
    >
      {show && children}
    </motion.div>
  );
}

/**
 * Slide up/down for accordions, dropdowns
 */
export function SlideTransition({ children, open, height = "auto" }: { children: ReactNode; open: boolean; height?: string | number }) {
  return (
    <motion.div
      initial={false}
      animate={{
        opacity: open ? 1 : 0,
        height: open ? height : 0,
      }}
      transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
      style={{ overflow: "hidden" }}
    >
      {children}
    </motion.div>
  );
}

/**
 * Scale + fade for popovers, tooltips
 */
export function PopoverTransition({ children, open }: { children: ReactNode; open: boolean }) {
  return (
    <motion.div
      initial={false}
      animate={open ? { opacity: 1, scale: 1 } : { opacity: 0, scale: 0.95 }}
      transition={{ duration: 0.12, ease: [0.16, 1, 0.3, 1] }}
    >
      {open && children}
    </motion.div>
  );
}

/**
 * Crossfade for content switching
 */
export function CrossfadeTransition({ children, key }: { children: ReactNode; key: string | number }) {
  return (
    <motion.div
      key={key}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
    >
      {children}
    </motion.div>
  );
}

/**
 * Layout transition for shared element animations
 */
export function LayoutTransition({ children }: { children: ReactNode }) {
  return (
    <motion.div layout>
      {children}
    </motion.div>
  );
}
