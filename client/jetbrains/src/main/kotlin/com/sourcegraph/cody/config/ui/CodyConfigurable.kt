package com.sourcegraph.cody.config.ui

import com.intellij.openapi.components.service
import com.intellij.openapi.options.BoundConfigurable
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.DialogPanel
import com.intellij.ui.ColorPanel
import com.intellij.ui.JBColor
import com.intellij.ui.components.JBCheckBox
import com.intellij.ui.dsl.builder.*
import com.intellij.ui.dsl.gridLayout.HorizontalAlign
import com.sourcegraph.cody.config.CodyApplicationSettings
import com.sourcegraph.cody.config.SettingsModel
import com.sourcegraph.cody.config.notification.CodySettingChangeActionNotifier
import com.sourcegraph.cody.config.notification.CodySettingChangeContext
import com.sourcegraph.cody.config.ui.lang.AutocompleteLanguageTable
import com.sourcegraph.cody.config.ui.lang.AutocompleteLanguageTableWrapper
import com.sourcegraph.config.ConfigUtil

class CodyConfigurable(val project: Project) : BoundConfigurable(ConfigUtil.CODY_DISPLAY_NAME) {
  private lateinit var dialogPanel: DialogPanel
  private val settingsModel = SettingsModel()
  private val codyApplicationSettings = service<CodyApplicationSettings>()

  override fun createPanel(): DialogPanel {
    dialogPanel = panel {
      lateinit var enableCodyCheckbox: Cell<JBCheckBox>
      group("Cody") {
        row {
          enableCodyCheckbox =
              checkBox("Enable Cody")
                  .comment(
                      "Disable this to turn off all AI-based functionality of the plugin, including the Cody chat sidebar and autocomplete",
                      MAX_LINE_LENGTH_NO_WRAP)
                  .bindSelected(settingsModel::isCodyEnabled)
        }
        row {
          checkBox("Enable debug")
              .comment("Enables debug output visible in the idea.log")
              .enabledIf(enableCodyCheckbox.selected)
              .bindSelected(settingsModel::isCodyDebugEnabled)
        }
        row {
          checkBox("Verbose debug")
              .enabledIf(enableCodyCheckbox.selected)
              .bindSelected(settingsModel::isCodyVerboseDebugEnabled)
        }
      }

      group("Autocomplete") {
        lateinit var enableAutocompleteCheckbox: Cell<JBCheckBox>
        row {
          val enableCustomAutocompleteColor =
              checkBox("Custom color for completions")
                  .enabledIf(enableCodyCheckbox.selected)
                  .bindSelected(settingsModel::isCustomAutocompleteColorEnabled)
          colorPanel()
              .bind(
                  ColorPanel::getSelectedColor,
                  ColorPanel::setSelectedColor,
                  settingsModel::customAutocompleteColor.toMutableProperty())
              .visibleIf(enableCustomAutocompleteColor.selected)
        }
        row {
          enableAutocompleteCheckbox =
              checkBox("Automatically trigger completions")
                  .enabledIf(enableCodyCheckbox.selected)
                  .bindSelected(settingsModel::isCodyAutocompleteEnabled)
        }
        row {
          autocompleteLanguageTable()
              .enabledIf(enableAutocompleteCheckbox.selected)
              .horizontalAlign(HorizontalAlign.FILL)
              .bind(
                  AutocompleteLanguageTableWrapper::getBlacklistedLanguageIds,
                  AutocompleteLanguageTableWrapper::setBlacklistedLanguageIds,
                  settingsModel::blacklistedLanguageIds.toMutableProperty())
        }
      }
    }
    return dialogPanel
  }

  override fun reset() {
    settingsModel.isCodyEnabled = codyApplicationSettings.isCodyEnabled
    settingsModel.isCodyAutocompleteEnabled = codyApplicationSettings.isCodyAutocompleteEnabled
    settingsModel.isCodyDebugEnabled = codyApplicationSettings.isCodyDebugEnabled
    settingsModel.isCodyVerboseDebugEnabled = codyApplicationSettings.isCodyVerboseDebugEnabled
    settingsModel.isCustomAutocompleteColorEnabled =
        codyApplicationSettings.isCustomAutocompleteColorEnabled
    settingsModel.customAutocompleteColor =
        // note: this sets the same value for both light & dark mode, currently
        codyApplicationSettings.customAutocompleteColor?.let { JBColor(it, it) }
    settingsModel.blacklistedLanguageIds = codyApplicationSettings.blacklistedLanguageIds
    dialogPanel.reset()
  }

  override fun apply() {
    val bus = project.messageBus
    val publisher = bus.syncPublisher(CodySettingChangeActionNotifier.TOPIC)
    super.apply()
    val context =
        CodySettingChangeContext(
            codyApplicationSettings.isCodyEnabled,
            settingsModel.isCodyEnabled,
            codyApplicationSettings.isCodyAutocompleteEnabled,
            settingsModel.isCodyAutocompleteEnabled,
            codyApplicationSettings.isCustomAutocompleteColorEnabled,
            settingsModel.isCustomAutocompleteColorEnabled,
            codyApplicationSettings.customAutocompleteColor,
            settingsModel.customAutocompleteColor?.rgb,
            codyApplicationSettings.blacklistedLanguageIds,
            settingsModel.blacklistedLanguageIds)
    codyApplicationSettings.isCodyEnabled = settingsModel.isCodyEnabled
    codyApplicationSettings.isCodyAutocompleteEnabled = settingsModel.isCodyAutocompleteEnabled
    codyApplicationSettings.isCodyDebugEnabled = settingsModel.isCodyDebugEnabled
    codyApplicationSettings.isCodyVerboseDebugEnabled = settingsModel.isCodyVerboseDebugEnabled
    codyApplicationSettings.isCustomAutocompleteColorEnabled =
        settingsModel.isCustomAutocompleteColorEnabled
    codyApplicationSettings.customAutocompleteColor = settingsModel.customAutocompleteColor?.rgb
    codyApplicationSettings.blacklistedLanguageIds = settingsModel.blacklistedLanguageIds

    publisher.afterAction(context)
  }
}

fun Row.colorPanel() = cell(ColorPanel())

fun Row.autocompleteLanguageTable() = cell(AutocompleteLanguageTable().wrapperComponent)
