import { createContext, useContext, useState } from "react";

const TabsContext = createContext();

export function Tabs({ children, defaultValue }) {
  const [activeTab, setActiveTab] = useState(defaultValue);

  return (
    <TabsContext.Provider value={{ activeTab, setActiveTab }}>
      <div className="w-full">
        {children}
      </div>
    </TabsContext.Provider>
  );
}

export function TabsList({ children, className = "" }) {
  return <div className={`flex gap-0 border-b border-gray-200 dark:border-gray-700 ${className}`}>{children}</div>;
}

export function TabsTrigger({ value, children, className = "", ...props }) {
  const { activeTab, setActiveTab } = useContext(TabsContext);
  const isActive = activeTab === value;

  return (
    <button
      {...props}
      onClick={() => setActiveTab(value)}
      className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
        isActive
          ? "border-blue-600 text-blue-600 dark:text-blue-400"
          : "border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-300"
      } ${className}`}
    >
      {children}
    </button>
  );
}

export function TabsContent({ value, children }) {
  const { activeTab } = useContext(TabsContext);

  if (activeTab !== value) return null;

  return <div>{children}</div>;
}
