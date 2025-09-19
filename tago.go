package tago

import (
	"reflect"
	"strings"
)

// Create a struct to handle custom tags
// For example, to handle a tag named "gorm2"
//
// Usage:
//
// 	type MyModel struct {
//  	   Field1 string `gorm2:"preload=true;otherOption=value"`
// 	}
// 	t := TaGo{Name: "gorm2"}
// 	tags := t.GetTags(&MyModel{})
// 	fmt.Println(tags) // map[preload=true:[Field1] otherOption=value:[Field1]]
type TaGo struct {
	Name string
}

// Ex: "preload=true" -> [Field1, Field1.Subfield2, ..]
type Instructions map[Instruction][]FieldName

// ex: preload=true
type Instruction string

func (i Instruction) Key() string {
	parts := strings.SplitN(string(i), "=", 2)
	return strings.TrimSpace(parts[0])
}

// Return the value of the instruction, or "true" if no value is provided
func (i Instruction) Value() string {
	parts := strings.SplitN(string(i), "=", 2)
	if len(parts) > 1 {
		return strings.TrimSpace(parts[1])
	}

	// If no value is provided, we consider it to be "true"
	return "true"
}

// ex: Field1, Field1.Subfield2
type FieldName string

func (f FieldName) AddPrefix(prefix string) FieldName {
	return FieldName(prefix + string(f))
}
func (f FieldName) String() string {
	return string(f)
}


func (t *Instructions) concat(other Instructions, prefix string) {
	for key, values := range other {
		if _, exists := (*t)[key]; !exists {
			(*t)[key] = make([]FieldName, 0)
		} 

		for _, v := range values {
			(*t)[key] = append((*t)[key], v.AddPrefix(prefix))
		}
	}
}


// From a model field, extract the custom tag and return a map of instructions to field names
// Model field is of type reflect.StructField Name - Tags
func (t TaGo) GetFromField(modelField reflect.StructField) Instructions{
	tags := make(Instructions)

	// Extract the t.Name:"tag1=value1;tag2=value2" part
	if tagsAsString := modelField.Tag.Get(t.Name); tagsAsString != "" {

		// We have all the values for this tag, so we need to split them by ';'
		instructions := strings.SplitSeq(tagsAsString, ";")
		for instruction := range instructions {
			// Extract key and value, e.g. "preload=true"
			parts := strings.SplitN(instruction, "=", 2)

			// Remove any extra spaces
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}

			// Join back with '=' in case the value had '=' in it
			instructionString := strings.Join(parts, "=")
			
			// If the tag value is empty, skip it
			if instructionString == "" {
				continue
			}

			instruction := Instruction(instructionString)

			// If instruction doesn't already exist, create it
			if _, exists := tags[instruction]; !exists {
				tags[instruction] = make([]FieldName, 0)
			}

			// Add the field name to the list of fields for this instruction
			tags[instruction] = append(tags[instruction], FieldName(modelField.Name))
		}
	}

	return tags
}

// Get the element type if it's a pointer or slice
// E.g. *T -> T, []T -> T, []*T -> T
func typeToElem(t reflect.Type) reflect.Type {
	// If it's a pointer, get the element type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// If it's a slice, get the element type
	if t.Kind() == reflect.Slice {
		t = t.Elem()

		// If it's a pointer, get the element type ([] *T)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
	}
	return t
}

// Get all the custom tags from a model, non-nested (only the top-level fields)
//
// Example:
// 	type MyModel struct {
//  	   Field1 string `gorm2:"preload=true;otherOption=value"`
//  	   Field2 int
//  	   Field3 NestedModel `gorm2:"preload=true"`
// 	}
// 	type NestedModel struct {
// 	    Subfield1 string `gorm2:"otherOption=value2"`
// 	}
// 	t := TaGo{Name: "gorm2"}
// 	tags := t.Get(&MyModel{})
// 	fmt.Println(tags) // map[preload=true:[Field1 Field3] otherOption=value:[Field1]]]
func (t TaGo) Get(model interface{}) Instructions {
	tags := make(Instructions)

	modelType := reflect.TypeOf(model)

	// Get the element type if it's a pointer or slice
	modelType = typeToElem(modelType)

	for i := 0; i < modelType.NumField(); i++ {
		modelField := modelType.Field(i)

		// Extract the t.Name tag for the current model field
		if fieldTags := t.GetFromField(modelField); fieldTags != nil {
			tags.concat(fieldTags, "")
		}
	}
	return tags
}

// Recursive function to get nested fields
func (t TaGo) getNested(model interface{}, prefix string, separator string) Instructions{
	tags := make(Instructions)
	
	modelType := reflect.TypeOf(model)
	// Get the element type if it's a pointer or slice
	modelType = typeToElem(modelType)

	for i := 0; i < modelType.NumField(); i++ {
		modelField := modelType.Field(i)

		// Extract the custom tag from the current field and add it to the tags slice
		if fieldTags := t.GetFromField(modelField); fieldTags != nil {
			tags.concat(fieldTags, prefix)
		}

		// If it's a struct, get its nested fields recursively too
		
		// Get the element type if it's a pointer or slice
		modelField.Type = typeToElem(modelField.Type)

		if modelField.Type.String() != modelType.String() { // Avoid infinite recursion on self-referencing structs
			if modelField.Type.Kind() == reflect.Struct {
				// Get the nested fields with updated prefix, and append them to the main tags slice
				t := t.getNested(reflect.New(modelField.Type).Elem().Interface(), prefix + modelField.Name+separator, separator)

				// Concat the nested tags (prefix has already been added in the recursive call)
				tags.concat(t, "")
			}
		}

	}
	return tags
}


// GetNested returns all custom tags from a model, including nested structs
// The nested struct fields will have their names prefixed with the parent field name and the separator.
//
// Example:
// 	type MyModel struct {
//  	   Field1 string `gorm2:"preload=true;otherOption=value"`
//  	   Field2 int
//  	   Field3 NestedModel `gorm2:"preload=true"`
// 	}
// 	type NestedModel struct {
// 	    Subfield1 string `gorm2:"preload=true;otherOption=value2"`
// 	}
// 	t := TaGo{Name: "gorm2"}
// 	tags := t.GetNested(&MyModel{}, ".")
// 	fmt.Println(tags) // map[preload=true:[Field1 Field3 Field3.SubField1] otherOption=value:[Field1] otherOption=value2:[Field3.Subfield1]]]
func (t TaGo) GetNested(model interface{}, separator string) Instructions {
	return t.getNested(model, "", separator)
}


// Apply the given instructions to the provided mapping of instruction to action function
// For each instruction in the instructions map, if it exists in the mapping, call the corresponding function for each field
//
// Example usage:
// 	instructions := t.Get(&MyModel{})
// 	instructionMapping := map[Instruction]func (field FieldName) {
// 	    "preload=true": func (field FieldName) { 
// 			fmt.Println("Preloading", field) 
// 		},
// 		"otherOption=value": func (field FieldName) {
// 	  			fmt.Println("Other option for", field)
// 		},
// 	}
// 	t.Apply(instructions, instructionMapping)
func (t TaGo) Apply(instructions Instructions, instructionMapping map[Instruction]func (field FieldName)) {
	for instruction, action := range instructionMapping {
		if fields, exists := instructions[instruction]; exists {
			for _, field := range fields {
				action(field)
			}
		}
	}
}

// ApplyOne applies a single instruction if it exists in the instructions map
// 
// Example usage:
// 	instructions := t.Get(&MyModel{})
// 	t.ApplyOne(Instruction("preload=true"), instructions, func(field FieldName) {
// 	    fmt.Println("Preloading", field)
// 	})
func (t TaGo) ApplyOne(instructionToCheck Instruction, instructions Instructions, action func(field FieldName)) {
	if fields, exists := instructions[instructionToCheck]; exists {
		for _, field := range fields {
			action(field)
		}
	}
}

// Check if a specific instruction exists in the instructions map
func (t TaGo) Has(model interface{}, instructionToCheck Instruction) bool {
	instructions := t.Get(model)
	_, exists := instructions[instructionToCheck]
	return exists
}

