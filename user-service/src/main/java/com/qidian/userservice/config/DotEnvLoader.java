package com.qidian.userservice.config;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.function.BiConsumer;
import java.util.function.Function;

public final class DotEnvLoader {

    private DotEnvLoader() {
    }

    public static void load(Path path) {
        load(path, DotEnvLoader::existingValue, (key, value) -> System.setProperty(key, value));
    }

    static void load(Path path, Function<String, String> existingValue, BiConsumer<String, String> setValue) {
        if (!Files.isRegularFile(path)) {
            return;
        }
        try (BufferedReader reader = Files.newBufferedReader(path)) {
            String line;
            while ((line = reader.readLine()) != null) {
                loadLine(line, existingValue, setValue);
            }
        } catch (IOException ex) {
            throw new IllegalStateException("Failed to load .env file: " + path, ex);
        }
    }

    private static void loadLine(String rawLine, Function<String, String> existingValue, BiConsumer<String, String> setValue) {
        String line = rawLine.trim();
        if (line.isEmpty() || line.startsWith("#")) {
            return;
        }
        if (line.startsWith("export ")) {
            line = line.substring("export ".length()).trim();
        }
        int separatorIndex = line.indexOf('=');
        if (separatorIndex <= 0) {
            return;
        }
        String key = line.substring(0, separatorIndex).trim();
        String value = unquote(line.substring(separatorIndex + 1).trim());
        if (key.isEmpty() || hasText(existingValue.apply(key))) {
            return;
        }
        setValue.accept(key, value);
    }

    private static String existingValue(String key) {
        String systemProperty = System.getProperty(key);
        if (hasText(systemProperty)) {
            return systemProperty;
        }
        return System.getenv(key);
    }

    private static String unquote(String value) {
        if (value.length() >= 2) {
            char first = value.charAt(0);
            char last = value.charAt(value.length() - 1);
            if ((first == '"' && last == '"') || (first == '\'' && last == '\'')) {
                return value.substring(1, value.length() - 1);
            }
        }
        return value;
    }

    private static boolean hasText(String value) {
        return value != null && !value.trim().isEmpty();
    }
}